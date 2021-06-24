[![Bonsai Asset Badge](https://img.shields.io/badge/Sensu%20RocketChat%20Handler-Download%20Me-brightgreen.svg?colorB=89C967&logo=sensu)](https://bonsai.sensu.io/assets/sensu/sensu-rocketchat-handler) [![Build Status](https://travis-ci.org/sensu/sensu-rocketchat-handler.svg?branch=master)](https://travis-ci.org/sensu/sensu-rocketchat-handler)

# Sensu RocketChat Handler

- [Overview](#overview)
- [Usage examples](#usage-examples)
  - [Help output](#help-output)
  - [Environment variables](#environment-variables)
  - [Templates](#templates)
  - [Annotations](#annotations)
- [Configuration](#configuration)
  - [Asset registration](#asset-registration)
  - [Handler definition](#handler-definition)
  - [Check definition](#check-definition)
- [Installation from source and contributing](#installation-from-source-and-contributing)

## Overview


The [Sensu RocketChat Handler][0] is a [Sensu Event Handler][3] that sends event data
to a configured RocketChat channel.

## Usage examples

### Help output

Help:

```
Usage:
  sensu-rocketchat-handler [flags]
  sensu-rocketchat-handler [command]

Available Commands:
  help        Help about any command
  version     Print the version number of this plugin

Flags:
  -c, --channel string                RocketChat channel to send messages to. (Required)
  -u, --url string                    RocketChat service URL (default "http://localhost:3000")
  -t, --description-template string   The RocketChat notification output template, in Golang text/template format (default "{{ .Check.Output }}")
      --alias string                  Name to use in the RocketChat msg. (Note: user must have bot role to take effect) (default "sensu")
      --avatar-url string             Avatar image url to use in RocketChat msg. (Note: user must have bot role to take effect) (default "https://www.sensu.io/img/sensu-logo.png")
  -P, --password string               RocketChat User Password. Used with --user. Note for security using ROCKETCHAT_PASSWORD environment variable is preferred
  -U, --user string                   RocketChat User. Used with --password. Note for security using ROCKETCHAT_USER environment variable is preferred
  -T, --token string                  RocketChat Auth Token. Used with --userID Note for security using ROCKETCHAT_TOKEN environment variable is preferred
  -I, --userID string                 RocketChat Auth UserID. Used with --token. Note for security using ROCKETCHAT_USERID environment variable is preferred
  -v, --verbose                       Verbose output
  -n, --dry-run                       Used for testing, do not communicate with RocketChat API, report only (implies --verbose)
  -h, --help                          help for sensu-rocketchat-handler
```

### Environment variables

|Argument               |Environment Variable            |
|-----------------------|--------------------------------|
|--channel              |ROCKETCHAT_CHANNEL              |
|--url                  |ROCKETCHAT_URL                  |
|--username             |ROCKETCHAT_USER                 |
|--password             |ROCKETCHAT_PASSWORD             |
|--token                |ROCKETCHAT_TOKEN                |
|--userID               |ROCKETCHAT_USERID               |
|--alias                |ROCKETCHAT_ALIAS                |
|--avatar-url           |ROCKETCHAT_AVATAR_URL           |
|--description-template |ROCKETCHAT_DESCRIPTION_TEMPLATE |


**Security Note:** Care should be taken to not expose the user/password or token/userID authentication 
credentials for this handler by specifying them on the command line or by directly setting the environment 
variable in the handler definition.  It is suggested to make use of [secrets management][7] to surface these as 
an environment variables.  The handler definition referenced here uses secrets for the authentication credentials.
Below is an example secrets definition that make use of the built-in [env secrets provider][8].

```yml
---
type: Secret
api_version: secrets/v1
metadata:
  name: rocketchat_user
spec:
  provider: env
  id: ROCKETCHAT_USER
---
type: Secret
api_version: secrets/v1
metadata:
  name: rocketchat_password
spec:
  provider: env
  id: ROCKETCHAT_PASSWORD
---
type: Secret
api_version: secrets/v1
metadata:
  name: rocketchat_token
spec:
  provider: env
  id: ROCKETCHAT_TOKEN
---
type: Secret
api_version: secrets/v1
metadata:
  name: rocketchat_userid
spec:
  provider: env
  id: ROCKETCHAT_USERID
```

### Templates

This handler provides options for using templates to populate the SumoLogic HTTP source attributes. 
By default the source host is populated using a template to extract the Sensu entity name. The default
source name is populated using a template to extract the Sensu check name. More information on template 
syntax and format can be found in [the documentation][9]


### Annotations

All arguments for this handler are tunable on a per entity or check basis based
on annotations. The annotations keyspace for this handler is
`sensu.io/plugins/rocketchat/config`.

**NOTE**: Due to [check token substituion][10], supplying a template value such
as for `description-template` as a check annotation requires that you place the
desired template as a [golang string literal][11] (enlcosed in backticks)
within another template definition.  This does not apply to entity annotations.

#### Examples

To customize the channel for a given entity, you could use the following
sensu-agent configuration snippet:

```yml
# /etc/sensu/agent.yml example
annotations:
  sensu.io/plugins/rocketchat/config/channel: '#monitoring'
```

## Configuration

### Asset registration

Assets are the best way to make use of this handler. If you're not using an asset, please consider doing so! If you're using sensuctl 5.13 or later, you can use the following command to add the asset:

`sensuctl asset add sensu/sensu-rocketchat-handler`

If you're using an earlier version of sensuctl, you can download the asset
definition from [this project's Bonsai Asset Index
page][6].

### Handler definition

Create the handler using the following handler definition:

```yml
---
api_version: core/v2
type: Handler
metadata:
  namespace: default
  name: rocketchat
spec:
  type: pipe
  command: sensu-rocketchat-handler --channel '#general' --url 'http://rocketchat.yourdomain.com'
  filters:
  - is_incident
  runtime_assets:
  - sensu/sensu-rocketchat-handler
  secrets:
  - name: ROCKETCHAT_PASSWORD
    secret: rocketchat_password
  - name: ROCKETCHAT_USER
    secret: rocketchat_user
  timeout: 10
```

**Security Note**: The Rocketchat authentication credentials user/password or token/userID
should always be treated as a security sensitive configuration options and in this example, 
they are loaded into the handler configuration as an environment variable using a [secret][5]. 
Command arguments are commonly readable from the process table by other unprivaledged
users on a system (ex: ps and top commands), so it's a better practise to read
in sensitive information via environment variables or configuration files on
disk. The cmdline argument flags for credentials are provided as overrides for testing purposes.

### Check definition

```
api_version: core/v2
type: CheckConfig
metadata:
  namespace: default
  name: dummy-app-healthz
spec:
  command: check-http -u http://localhost:8080/healthz
  subscriptions:
  - dummy
  publish: true
  interval: 10
  handlers:
  - rocketchat
```

### Proxy Support

This handler supports the use of the environment variables HTTP_PROXY,
HTTPS_PROXY, and NO_PROXY (or the lowercase versions thereof). HTTPS_PROXY takes
precedence over HTTP_PROXY for https requests.  The environment values may be
either a complete URL or a "host[:port]", in which case the "http" scheme is assumed.

## Installing from source and contributing

Download the latest version of the sensu-rocketchat-handler from [releases][4],
or create an executable from this source.

### Compiling

From the local path of the sensu-rocketchat-handler repository:
```
go build
```

To contribute to this plugin, see [CONTRIBUTING](https://github.com/sensu/sensu-go/blob/master/CONTRIBUTING.md)

[0]: https://github.com/sensu/sensu-rocketchat-handler
[1]: https://github.com/sensu/sensu-go
[3]: https://docs.sensu.io/sensu-go/latest/reference/handlers/#how-do-sensu-handlers-work
[4]: https://github.com/sensu/sensu-rocketchat-handler/releases
[5]: https://docs.sensu.io/sensu-go/latest/reference/secrets/
[6]: https://bonsai.sensu.io/assets/sensu/sensu-rocketchat-handler
[7]: https://docs.sensu.io/sensu-go/latest/guides/secrets-management/
[8]: https://docs.sensu.io/sensu-go/latest/guides/secrets-management/#use-env-for-secrets-management
[9]: https://docs.sensu.io/sensu-go/latest/observability-pipeline/observe-process/handler-templates/
[10]: https://docs.sensu.io/sensu-go/latest/observability-pipeline/observe-schedule/checks/#check-token-substitution
[11]: https://golang.org/ref/spec#String_literals
