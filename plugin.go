/*
 * Copyright 2022 Aspect Build Systems, Inc. All rights reserved.
 *
 * Licensed under the aspect.build Community License (the "License");
 * you may not use this file except in compliance with the License.
 * Full License text is in the LICENSE file included in the root of this repository
 * and at https://aspect.build/communitylicense
 */
package main

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	goplugin "github.com/hashicorp/go-plugin"
	"gopkg.in/yaml.v2"

	"aspect.build/cli/bazel/buildeventstream"
	"aspect.build/cli/pkg/ioutils"
	"aspect.build/cli/pkg/plugin/sdk/v1alpha3/config"
	aspectplugin "aspect.build/cli/pkg/plugin/sdk/v1alpha3/plugin"
)

// main starts up the plugin as a child process of the CLI and connects the gRPC communication.
func main() {
	goplugin.Serve(config.NewConfigFor(&ErrorAugmentorPlugin{
		hintMap:             map[*regexp.Regexp]string{},
		yamlUnmarshalStrict: yaml.UnmarshalStrict,
		helpfulHints:        &helpfulHintSet{nodes: make(map[helpfulHintNode]struct{})},
	}))
}
type ErrorAugmentorPlugin struct {
	aspectplugin.Base

	hintMap                map[*regexp.Regexp]string
	yamlUnmarshalStrict    func(in []byte, out interface{}) (err error)
	helpfulHints           *helpfulHintSet
	helpfulHintsMutex      sync.Mutex
	errorMessages          chan string
	errorMessagesWaitGroup sync.WaitGroup
}

type pluginProperties struct {
	ErrorMappings  map[string]string `yaml:"error_mappings"`
	ProcessorCount int               `yaml:"processor_count"`
}

func (plugin *ErrorAugmentorPlugin) Setup(config *aspectplugin.SetupConfig) error {
	var properties pluginProperties

	if err := plugin.yamlUnmarshalStrict(config.Properties, &properties); err != nil {
		return fmt.Errorf("failed to setup: failed to parse properties: %w", err)
	}

	// change map keys into regex objects now so they are ready to use and we only need to compile the regex once
	for pattern, message := range properties.ErrorMappings {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return err
		}

		plugin.hintMap[regex] = message
	}

	plugin.errorMessages = make(chan string, 4096)

	processorCount := properties.ProcessorCount
	if processorCount == 0 {
		processorCount = 4
	}

	for i := 0; i < processorCount; i++ {
		go plugin.errorMessageProcessor()
		plugin.errorMessagesWaitGroup.Add(1)
	}

	return nil
}

func (plugin *ErrorAugmentorPlugin) BEPEventCallback(event *buildeventstream.BuildEvent) error {
	aborted := event.GetAborted()
	if aborted != nil {
		plugin.errorMessages <- aborted.Description

		// We exit early here because there will not be a progress message when the event was of type "aborted".
		return nil
	}

	progress := event.GetProgress()

	if progress != nil {
		stdout := progress.GetStdout()
		if stdout != "" {
			if err := plugin.queueErrorMessage(stdout); err != nil {
				return fmt.Errorf("failed to process build event: %w", err)
			}
		}

		stderr := progress.GetStderr()
		if stderr != "" {
			if err := plugin.queueErrorMessage(stderr); err != nil {
				return fmt.Errorf("failed to process build event: %w", err)
			}
		}
	}

	return nil
}

func (plugin *ErrorAugmentorPlugin) queueErrorMessage(errorMessage string) error {
	if len(plugin.errorMessages) == cap(plugin.errorMessages) {
		return fmt.Errorf("failed to queue error message: queue is full")
	}
	plugin.errorMessages <- errorMessage
	return nil
}

func (plugin *ErrorAugmentorPlugin) errorMessageProcessor() {
	for errorMessage := range plugin.errorMessages {
		plugin.processErrorMessage(errorMessage)
	}
	plugin.errorMessagesWaitGroup.Done()
}

func (plugin *ErrorAugmentorPlugin) processErrorMessage(errorMessage string) {
	for regex, helpfulHint := range plugin.hintMap {
		matches := regex.FindStringSubmatch(errorMessage)

		if len(matches) > 0 {

			// apply regex capture group replacements to given hint
			for i, match := range matches {
				if i == 0 {
					// skipping the first match because it will always contain the entire result
					// of the regex match. We are only after specific capture groups
					continue
				}
				helpfulHint = strings.ReplaceAll(helpfulHint, fmt.Sprint("$", i), match)
			}

			plugin.helpfulHintsMutex.Lock()
			plugin.helpfulHints.insert(helpfulHint)
			plugin.helpfulHintsMutex.Unlock()
		}
	}
}

func (plugin *ErrorAugmentorPlugin) PostBuildHook(
	isInteractiveMode bool,
	promptRunner ioutils.PromptRunner,
) error {
	close(plugin.errorMessages)
	plugin.errorMessagesWaitGroup.Wait()

	if plugin.helpfulHints.size == 0 {
		return nil
	}

	plugin.printBreak()

	plugin.printMiddle("[Error Augmentor Plugin]")
	plugin.printMiddle("")

	for node := plugin.helpfulHints.head; node != nil; node = node.next {
		plugin.printMiddle("- " + node.helpfulHint)
	}

	plugin.printBreak()
	return nil
}

func (plugin *ErrorAugmentorPlugin) printBreak() {
	// using buffer so that we can easily determine the current length of the string and
	// ensure we create a proper square with a straight border
	var b strings.Builder

	fmt.Fprintf(&b, " ")

	for i := 0; i < 90; i++ {
		fmt.Fprintf(&b, "-")
	}

	fmt.Fprintf(&b, " ")

	fmt.Println(b.String())
}

func (plugin *ErrorAugmentorPlugin) printMiddle(str string) {
	// using buffer so that we can easily determine the current length of the string and
	// ensure we create a proper square with a straight border
	var b strings.Builder

	fmt.Fprintf(&b, "| ")
	fmt.Fprintf(&b, str)

	for b.Len() < 91 {
		fmt.Fprintf(&b, " ")
	}

	fmt.Fprintf(&b, "|")
	fmt.Println(b.String())
}

func (plugin *ErrorAugmentorPlugin) PostTestHook(
	isInteractiveMode bool,
	promptRunner ioutils.PromptRunner,
) error {
	return plugin.PostBuildHook(isInteractiveMode, promptRunner)
}

func (plugin *ErrorAugmentorPlugin) PostRunHook(
	isInteractiveMode bool,
	promptRunner ioutils.PromptRunner,
) error {
	return plugin.PostBuildHook(isInteractiveMode, promptRunner)
}

type helpfulHintSet struct {
	head  *helpfulHintNode
	tail  *helpfulHintNode
	nodes map[helpfulHintNode]struct{}
	size  int
}

func (s *helpfulHintSet) insert(helpfulHint string) {
	node := helpfulHintNode{
		helpfulHint: helpfulHint,
	}
	if _, exists := s.nodes[node]; !exists {
		s.nodes[node] = struct{}{}
		if s.head == nil {
			s.head = &node
		} else {
			s.tail.next = &node
		}
		s.tail = &node
		s.size++
	}
}

type helpfulHintNode struct {
	next        *helpfulHintNode
	helpfulHint string
}
