# Plugin Starter Template

[![Build Status](https://github.com/mattermost/mattermost-plugin-starter-template/actions/workflows/ci.yml/badge.svg)](https://github.com/mattermost/mattermost-plugin-starter-template/actions/workflows/ci.yml)
[![E2E Status](https://github.com/mattermost/mattermost-plugin-starter-template/actions/workflows/e2e.yml/badge.svg)](https://github.com/mattermost/mattermost-plugin-starter-template/actions/workflows/e2e.yml)

This plugin serves as a starting point for writing a Mattermost plugin. Feel free to base your own plugin off this repository.

To learn more about plugins, see [our plugin documentation](https://developers.mattermost.com/extend/plugins/).

This template requires node v16 and npm v8. You can download and install nvm to manage your node versions by following the instructions [here](https://github.com/nvm-sh/nvm). Once you've setup the project simply run `nvm i` within the root folder to use the suggested version of node.

## Getting Started
Use GitHub's template feature to make a copy of this repository by clicking the "Use this template" button.

Alternatively shallow clone the repository matching your plugin name:
```
git clone --depth 1 https://github.com/mattermost/mattermost-plugin-starter-template com.example.my-plugin
```

Note that this project uses [Go modules](https://github.com/golang/go/wiki/Modules). Be sure to locate the project outside of `$GOPATH`.

Edit the following files:
1. `plugin.json` with your `id`, `name`, and `description`:
```json
{
    "id": "com.example.my-plugin",
    "name": "My Plugin",
    "description": "A plugin to enhance Mattermost."
}
```

2. `go.mod` with your Go module path, following the `<hosting-site>/<repository>/<module>` convention:
```
module github.com/example/my-plugin
```

3. `.golangci.yml` with your Go module path:
```yml
linters-settings:
  # [...]
  goimports:
    local-prefixes: github.com/example/my-plugin
```

Build your plugin:
```
make
```

This will produce a single plugin file (with support for multiple architectures) for upload to your Mattermost server:

```
dist/com.example.my-plugin.tar.gz
```

## Development

To avoid having to manually install your plugin, build and deploy your plugin using one of the following options. In order for the below options to work, you must first enable plugin uploads via your config.json or API and restart Mattermost.

```json
    "PluginSettings" : {
        ...
        "EnableUploads" : true
    }
```

### Development guidance 

1. Fewer packages is better: default to the main package unless there's good reason for a new package.

2. Coupling implies same package: don't jump through hoops to break apart code that's naturally coupled.

3. New package for a new interface: a classic example is the sqlstore with layers for monitoring performance, caching and mocking.

4. New package for upstream integration: a discrete client package for interfacing with a 3rd party is often a great place to break out into a new package

### Modifying the server boilerplate

The server code comes with some boilerplate for creating an api, using slash commands, accessing the kvstore and using the cluster package for jobs. 

#### Api

api.go implements the ServeHTTP hook which allows the plugin to implement the http.Handler interface. Requests destined for the `/plugins/{id}` path will be routed to the plugin. This file also contains a sample `HelloWorld` endpoint that is tested in plugin_test.go.

#### Command package

This package contains the boilerplate for adding a slash command and an instance of it is created in the `OnActivate` hook in plugin.go. If you don't need it you can delete the package and remove any reference to `commandClient` in plugin.go. The package also contains an example of how to create a mock for testing.

#### KVStore package

This is a central place for you to access the KVStore methods that are available in the `pluginapi.Client`. The package contains an interface for you to define your methods that will wrap the KVStore methods. An instance of the KVStore is created in the `OnActivate` hook.

### Deploying with Local Mode

If your Mattermost server is running locally, you can enable [local mode](https://docs.mattermost.com/administration/mmctl-cli-tool.html#local-mode) to streamline deploying your plugin. Edit your server configuration as follows:

```json
{
    "ServiceSettings": {
        ...
        "EnableLocalMode": true,
        "LocalModeSocketLocation": "/var/tmp/mattermost_local.socket"
    },
}
```

and then deploy your plugin:
```
make deploy
```

You may also customize the Unix socket path:
```bash
export MM_LOCALSOCKETPATH=/var/tmp/alternate_local.socket
make deploy
```

If developing a plugin with a webapp, watch for changes and deploy those automatically:
```bash
export MM_SERVICESETTINGS_SITEURL=http://localhost:8065
export MM_ADMIN_TOKEN=j44acwd8obn78cdcx7koid4jkr
make watch
```

### Deploying with credentials

Alternatively, you can authenticate with the server's API with credentials:
```bash
export MM_SERVICESETTINGS_SITEURL=http://localhost:8065
export MM_ADMIN_USERNAME=admin
export MM_ADMIN_PASSWORD=password
make deploy
```

or with a [personal access token](https://docs.mattermost.com/developer/personal-access-tokens.html):
```bash
export MM_SERVICESETTINGS_SITEURL=http://localhost:8065
export MM_ADMIN_TOKEN=j44acwd8obn78cdcx7koid4jkr
make deploy
```

## Maestro AI Integration

This plugin uses Maestro to execute configured AI tasks with context from chat messages. You can interact with Maestro using the `/maestro` slash command.

### Usage

To execute an AI task with Maestro, type the following command and press Enter:
```
/maestro <task_name> <num_messages>
```
*   `<task_name>`: This must match one of the task names you define in the "Maestro Task Definitions (JSON)" setting in the plugin configuration (see below).
*   `<num_messages>`: This specifies how many recent messages from the current channel should be fetched and provided as context to the AI for the task. It must be a positive integer.

For example, if you have a task named `summarize_conversation` defined in your configuration, you can run:
```
/maestro summarize_conversation 15
```
This command will fetch the last 15 messages from the current channel and use them, along with the prompt template defined for `summarize_conversation`, to generate a response from the AI. The result will be posted back to the channel by the plugin bot.

### Configuration

To enable and customize AI task execution, the following settings must be configured in the System Console under **Plugins > [Your Plugin's Display Name]** (e.g., "Maestro" or "Agentic Mattermost"):

1.  **OpenAI API Key** (Required):
    *   This is your secret API key from OpenAI. It is essential for the plugin to communicate with the OpenAI API.
    *   You can obtain an API key from [https://platform.openai.com/account/api-keys](https://platform.openai.com/account/api-keys).
    *   Without a valid API key, the plugin will not be able to execute AI tasks.

2.  **OpenAI Model** (Optional, has default):
    *   Specify the OpenAI model to use for generating responses (e.g., `gpt-3.5-turbo`, `gpt-4`, `gpt-4-turbo-preview`).
    *   If left blank or the specified model is invalid, the plugin defaults to `gpt-3.5-turbo`.

3.  **Maestro Task Definitions (JSON)** (Required for functionality):
    *   This setting allows you to define custom AI tasks that users can invoke with the `/maestro` command.
    *   It must be a valid JSON object where:
        *   Each **key** is a unique `task_name` (e.g., "summarize_chat", "extract_actions", "brainstorm_ideas"). This `task_name` is what users will type in the `/maestro` command.
        *   Each **value** is a string representing the prompt template for that task.
    *   **Crucially, within each prompt template, you *must* include the `{{.Messages}}` placeholder.** This placeholder will be automatically replaced by the plugin with the concatenated content of the fetched chat messages.
    *   Example JSON:
        ```json
        {
          "summarize_chat": "Please provide a concise summary of the following conversation, highlighting key decisions and action items:\n\n{{.Messages}}",
          "extract_actions": "From the discussion below, list all specific action items, who is assigned to them (if mentioned), and any deadlines:\n\n{{.Messages}}",
          "brainstorm_solutions": "Given this problem description, brainstorm three potential solutions:\n\n{{.Messages}}"
        }
        ```
    *   If a `task_name` used in the `/maestro` command is not found in this configuration, users will receive an error. If the JSON is malformed, parsing errors will be logged by the plugin.

To enable AI task execution, you must configure the OpenAI API Key (as Maestro currently uses OpenAI as its backend):

<!-- The detailed steps for obtaining and setting API key were integrated into the list above -->

Without a valid API key, the plugin will not be able to execute AI tasks and will log an error. If task definitions are missing or malformed, specific tasks may not work.

### Releasing new versions

The version of a plugin is determined at compile time, automatically populating a `version` field in the [plugin manifest](plugin.json):
* If the current commit matches a tag, the version will match after stripping any leading `v`, e.g. `1.3.1`.
* Otherwise, the version will combine the nearest tag with `git rev-parse --short HEAD`, e.g. `1.3.1+d06e53e1`.
* If there is no version tag, an empty version will be combined with the short hash, e.g. `0.0.0+76081421`.

To disable this behaviour, manually populate and maintain the `version` field.

## How to Release

To trigger a release, follow these steps:

1. **For Patch Release:** Run the following command:
    ```
    make patch
    ```
   This will release a patch change.

2. **For Minor Release:** Run the following command:
    ```
    make minor
    ```
   This will release a minor change.

3. **For Major Release:** Run the following command:
    ```
    make major
    ```
   This will release a major change.

4. **For Patch Release Candidate (RC):** Run the following command:
    ```
    make patch-rc
    ```
   This will release a patch release candidate.

5. **For Minor Release Candidate (RC):** Run the following command:
    ```
    make minor-rc
    ```
   This will release a minor release candidate.

6. **For Major Release Candidate (RC):** Run the following command:
    ```
    make major-rc
    ```
   This will release a major release candidate.

## Q&A

### How do I make a server-only or web app-only plugin?

Simply delete the `server` or `webapp` folders and remove the corresponding sections from `plugin.json`. The build scripts will skip the missing portions automatically.

### How do I include assets in the plugin bundle?

Place them into the `assets` directory. To use an asset at runtime, build the path to your asset and open as a regular file:

```go
bundlePath, err := p.API.GetBundlePath()
if err != nil {
    return errors.Wrap(err, "failed to get bundle path")
}

profileImage, err := ioutil.ReadFile(filepath.Join(bundlePath, "assets", "profile_image.png"))
if err != nil {
    return errors.Wrap(err, "failed to read profile image")
}

if appErr := p.API.SetProfileImage(userID, profileImage); appErr != nil {
    return errors.Wrap(err, "failed to set profile image")
}
```

### How do I build the plugin with unminified JavaScript?
Setting the `MM_DEBUG` environment variable will invoke the debug builds. The simplist way to do this is to simply include this variable in your calls to `make` (e.g. `make dist MM_DEBUG=1`).
