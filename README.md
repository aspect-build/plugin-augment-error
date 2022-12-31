# Augment Error plugin for Aspect CLI

This is a plugin for the [Aspect CLI](https://aspect.build/cli).

It matches on error messages from Bazel, and adds extra information that can help your engineers
such as golinks to your internal documentation, tell them that a migration is underway with
additional instructions, or whatever you can think of.

Users configure it in an `error_mappings` property in the `.aspect/cli/plugins.yaml` file in their repo.

## Demo

With a configuration like:

```yaml
- name: augment-error
  properties:
    error_mappings:
      demo: this message helps our devs understand failures with the string "demo"
```

This plugin will print the message when the error contains "demo", like the following:

[![asciicast](https://asciinema.org/a/540385.svg)](https://asciinema.org/a/540385)
