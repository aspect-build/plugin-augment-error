# Augment Error plugin for Aspect CLI

This is a plugin for the Aspect CLI.

It matches on error messages from Bazel, and adds extra information that can help your engineers
such as golinks to your internal documentation, tell them that a migration is underway with
additional instructions, or whatever you can think of.

You configure it in an `error_mappings` property in .aspect/cli/plugins.yaml file in your repo, like so:

```yaml
- name: augment-error
  properties:
    error_mappings:
      demo: this message helps our devs understand failures with the string "demo"
```

This will print the message when the error contains "demo", like the following:

[![asciicast](https://asciinema.org/a/540385.svg)](https://asciinema.org/a/540385)

## Developing

To try the plugin, first check that you have the most recent [aspect cli release] installed.

First build the plugin from source:

```bash
% bazel build ...
```

Note that the `.aspect/cli/plugins.yaml` file has a reference to the path under `bazel-bin` where the plugin binary was just written.
On the first build, you'll see a warning printed that the plugin doesn't exist at this path.
This is just the development flow for working on plugins; users will reference the plugin's releases which are downloaded for them automatically.

Now just run `aspect`. You should see that `hello-world` appears in the help output. This shows that our plugin was loaded and contributed a custom command to the CLI.

```
Usage:
  aspect [command]

Custom Commands from Plugins:
  hello-world        Print 'Hello World!' to the command line.
```

## Releasing

Just push a tag to your GitHub repo.
The actions integration will create a release.

[bazelisk]: https://bazel.build/install/bazelisk
[aspect cli]: https://aspect.build/cli
[plugin documentation]: https://docs.aspect.build/aspect-build/aspect-cli/5.0.1/docs/help/topics/plugins.html
[aspect cli release]: https://github.com/aspect-build/aspect-cli/releases
