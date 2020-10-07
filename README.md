# API Manager Tools

![GitHub tag (latest SemVer)](https://img.shields.io/github/v/tag/jake-scott/apim-tools)![Travis (.org)](https://img.shields.io/travis/jake-scott/apim-tools)
[![Go Report Card](https://goreportcard.com/badge/github.com/jake-scott/apim-tools?style=flat-square)](https://goreportcard.com/report/github.com/jake-scott/apim-tools)
![Go Version](https://img.shields.io/badge/go%20version-%3E=1.14-61CFDD.svg?style=flat-square)
[![PkgGoDev](https://pkg.go.dev/badge/mod/github.com/jake-scott/apim-tools)](https://pkg.go.dev/mod/github.com/jake-scott/apim-tools)
![GitHub](https://img.shields.io/github/license/jake-scott/apim-tools)

## Description

Tooling to manage and maintain Azure API Manager instances.

The only functions implemented so far are download, upload and reset for the New (open source)
Developer Portal.

## Installation

Either install from source :


    $ git clone https://github.com/jake-scott/apim-tools.git
    $ cd apim-tools
    $ go install


or have Go install it for you :

    $ go get -u github.com/jake-scott/apim-tools

The binary will be installed as `$GOPATH/bin/apim-tools` (usually ~/go/bin/apim-tools).


## Configuration

The tools will optionally read a configuration file.  The default location is `~/.apim-tools.yml`, though
that can be overridden with the `--config` option.

Two sections are supported:

### Logging

By default, log entries are sent to stderr in text mode and info level.  These defaults can be overridden
in the config file, eg :

```yaml
logging:
  location: stderr
  format: text
  level: info
```

.. where:

   *  _location_ may be stderr, stdout or the name of a file.
   *  _format_ may be text or json
   *  _level_ may be one of fatal, error, warn, info, debug or trace

Debug mode can also be enabled per execution with the `--debug` flag

## Authentication

The tools make use of Hashicorp's excellent Azure authentication wrappers.  That means
that they support the use of several ways to obtain credentials out of the box.

### From the `az` CLI tool

First install the [Azure CLI tools ](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli?view=azure-cli-latest), then authenticate :

    $ shell
    $ az login

Follow the prompts to authenticate using the browser, and the CLI will stash tokens in `~/.azure` that
apim-tools can use.  Only the `--subsctiption` authentication option needs to be supplied when running apim-tools.

### Using a Service Principal

An service principal can be used with a client-secret or certificate.  Certificates should be supplied as a PKCS12 bundle that includes the certificate and private key, and the file must have a `.pfx` extension.

**Using a client secret:**

   * Command line options `--client-id`, `--client-secret` and `--tenant`

or
   * Configuration file options:

```yaml
auth:
  client-id: e1f509c4-7c0d-4d0f-a504-2ae30928fa59
  client-secret: someVerySecretValue
  tenant: myapis.onmicrosoft.com
```

**Using a client certificate:**

   * Command line options `--client-id`, `--cert-file`, `--cert-password` and `--tenant`

or
   * Configuration file options:

```yaml
auth:
  client-id: e1f509c4-7c0d-4d0f-a504-2ae30928fa59
  cert-file: /etc/pkitls/private/az.pfx
  cert-password: somePassword
  tenant: myapis.onmicrosoft.com
```

## Downloading the portal contents

The `devportal download` command can be used to dump the Developer Portal contents to a 
Zip archive. The archive includes JSON data describing the structure and text content, and all
of the binary media stored in the Developer Portal's storage account.

The following options are required:

   * `--apim` The name of the API Manager instance
   * `--rg`  The name of the Azure resource group containing the API Manager instance
   * `--out`  The name of the Zip archive to write

The following options are optional:
   * `--force`  Overwrite an existing archive (default: false)

For example:

```console
$ apim-tools  devportal download --subscription 1d6ff69a-30cb-48ff-9cf9-aa128c4d62d2  --apim myapim --rg prodrg --out /var/tmp/apim.zip
Using config file: /home/jacob/.apim-tools.yml
INFO[0000] Querying instance
INFO[0001] Processing content items...
INFO[0001]   -> 16 page items
INFO[0002]   -> 22 document items
INFO[0002]   -> 2 layout items
INFO[0002]   -> 0 blogpost items
INFO[0002]   -> 6 blob items
INFO[0002]   -> 4 url items
INFO[0002]   -> 0 navigation items
INFO[0002]   -> 1 block items
INFO[0002]   -> Total 51 items
INFO[0002] Downloading media...
INFO[0003]   -> Total 1 blobs, 0 errors
```

The ZIP file can be explored using standard tools including Windows explorer or [the Linux unzip utility](http://www.info-zip.org/UnZip.html) :

```console
$ unzip -l /var/tmp/apim.zip
Archive:  /var/tmp/apim.zip
  Length      Date    Time    Name
---------  ---------- -----   ----
    84458  09-24-2020 21:31   data.json
    32476  09-24-2020 21:31   180ffbc0-507a-4c46-a8f7-a74c8a4b1e9d
    32476  09-24-2020 21:31   2c26202c-7b55-4b28-b1c1-0bb54f3238d1
    32476  09-24-2020 21:31   2c60863c-3e9c-4d7b-a33b-1cfb354fd17b
    32476  09-24-2020 21:31   588b2ad9-352f-457e-81a6-4ddc5f3b53c1
    32476  09-24-2020 21:31   5bada4c4-6713-44ae-9936-df6aa9d86796
    32476  09-24-2020 21:31   8e8a3cc7-d0ff-4d6e-9bd4-fd5ef95982a1
    32476  09-24-2020 21:31   9f4c5b14-da6f-4254-ba11-756dc6cd43f8
    32476  09-24-2020 21:31   a50fefae-ac53-4595-9be0-059414500c7a
    32476  09-24-2020 21:31   a6e2835c-2c4d-4cbb-849b-235d1b10a0b1
    32476  09-24-2020 21:31   b0b4e26d-c719-3720-c37c-cd4ec9f28657
    32476  09-24-2020 21:31   bc8b66d0-eead-434e-b6a9-ccbe2bdfe581
    32476  09-24-2020 21:31   e249134e-0035-4a78-9dbb-f1acde5206a5
    32476  09-24-2020 21:31   e6816975-c9bc-44e5-8e3b-199db256465f
    32476  09-24-2020 21:31   e6df3446-e2cd-4267-bd43-71dd1165eb04
    32476  09-24-2020 21:31   f05c2d24-49cc-49fe-b3d6-36ab607372fb
---------                     -------
   571598                     16 files
```

## Uploading the portal contents

The `devportal upload` command can be used to restore a previously downloaded Developer Portal 
archive.

The following options are required:

   * `--apim` The name of the API Manager instance
   * `--rg`  The name of the Azure resource group containing the API Manager instance
   * `--in`  The name of the Zip archive to upload

The following options are optional:

   * `--nodelete` Skip deletion of items that exist on the portal but are not present in the archive


For example:

```console
$ apim-tools  devportal upload ---subscription 1d6ff69a-30cb-48ff-9cf9-aa128c4d62d2  --apim myapim --rg prodrg  --in /var/tmp/apim.zip
Using config file: /home/jacob/.apim-tools.yml
INFO[0000] Querying instance
INFO[0000] Processing 51 content items
INFO[0011]   -> Total 51 items, 0 errors
INFO[0011] Processed 1 media blobs, 0 skipped, 0 errors
INFO[0011] Deleted 6 extra media blobs, 0 errors
INFO[0012] Deleted 1 extra content items, 0 errors
```

## Erasing the portal contents

The `devportal reset` command will delete all content and media from the Developer Portal.

The following options are required:

   * `--apim` The name of the API Manager instance
   * `--rg`  The name of the Azure resource group containing the API Manager instance

For example:

```console
$ apim-tools  devportal reset ---subscription 1d6ff69a-30cb-48ff-9cf9-aa128c4d62d2  --apim myapim --rg prodrg
INFO[0000] Querying instance
INFO[0000] Deleting portal content items
INFO[0007] Deleted 52 content items, 0 errors
INFO[0007] Deleting blobs
INFO[0007] Deleted 7 blobs, 0 errors
```

## Publishing the portal ##

The `devportal publish` command will publish the Developer Portal contents.

The following options are required:

   * `--apim` The name of the API Manager instance
   * `--rg`  The name of the Azure resource group containing the API Manager instance

The following options are optional:

   * `--wait` Wait for the portal publish to complete

For example:
```console
$ apim-tools  devportal publish  ---subscription 1d6ff69a-30cb-48ff-9cf9-aa128c4d62d2  --apim myapim --rg prodrg
INFO[0000] Querying instance
INFO[0001] Developer portal published
```


> **_NOTE:_**  The published portal version is represented by a date string that has only minute resolution.  The tool will wait for a minute change if a publish is requesed less than one minute since the previous version.


## Display the portal status ##

The `devportal status` command displays the Developer Portal status.

The following options are required:

   * `--apim` The name of the API Manager instance
   * `--rg`  The name of the Azure resource group containing the API Manager instance

The following options are optional:

   * `--json` Return the status as JSON, for use in scripting
  

For example:
```console
$ apim-tools  devportal status  ---subscription 1d6ff69a-30cb-48ff-9cf9-aa128c4d62d2  --apim myapim --rg prodrg
INFO[0000] Querying instance
 Is deployed: true
Published at: 06 Oct 20 21:11 EDT
Code version: 20200925173036
     Version: 0.14.1072.0
```

When an API Manager instance is first deployed, the Developer Portal is not accessible, and _Is deployed_ will be false.  Uploading content and publishing the portal will also deploy the portal.


## Display the portal endpoints ##

The `devportal endpoints` command displays the Developer Portal, Management API and Blob Storage endpoints.

The following options are required:

   * `--apim` The name of the API Manager instance
   * `--rg`  The name of the Azure resource group containing the API Manager instance

The following options are optional:

   * `--json` Return the status as JSON, for use in scripting


For example:
```console
$ apim-tools  devportal status  ---subscription 1d6ff69a-30cb-48ff-9cf9-aa128c4d62d2  --apim myapim --rg prodrg --json
INFO[0000] Querying instance
{
    "blobStorageUrl": "https://apimstjhf1thirmkwiaaa84h.blob.core.windows.net/content?sv=2017-04-17\u0026sr=c\u0026sig=nejJdo%2B4f75%2FkQr1A6I2q1kF9BOaN38IiWgjDGpWjPw%3D\u0026se=2020-10-14T21:32:55Z\u0026sp=rwdl",
    "devPortalUrl": "https://wibble123.developer.azure-api.net",
    "managementUrl": "https://wibble123.management.azure-api.net"
}
```

## Generate a Shared Access Signature (SAS) token ##

The `devportal sastoken` command generates a SAS token for the Administrator user, for use in scripts making
use of the Management API.

The following options are required:

   * `--apim` The name of the API Manager instance
   * `--rg`  The name of the Azure resource group containing the API Manager instance

The following options are optional:

   * `--json` Return the status as JSON, for use in scripting


For example:
```console
$ TOKEN=$(apim-tools  devportal sastoken  ---subscription 1d6ff69a-30cb-48ff-9cf9-aa128c4d62d2  --apim myapim --rg prodrg)
INFO[0000] Querying instance
$ echo $TOKEN
1&202010022203&fl09XywaTtpCa0J6rgFScLlOpnW9sdEaJY9nnud2jFtTjLlMU7dUrBIG+YehDg1XBmyCmmHjyiJGsQwK9Ruqw==
```
