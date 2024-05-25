G-Force
=======

A set of utilities that interact with the Salesforce.com API

apexcov
-------

A CLI tool that can be used in CI/CD pipelines with Salesforce.

```
apexcov [-strategy=<value>] [-config=<value>] [-packages=<value>]
  -config
        Path to SF org authentication information (config.json) (default "config.json")
  -package
        Comma-separated list of paths to manifest (package.xml) (default "package.xml")
  -strategy
        Choose the strategy of getting coverage (default "MaxCoverage"):
          - "MaxCoverage" to ouput all tests that provide coverage for the passed in Apex
          - "MaxCoverageWithDeps" to output all tests for the passed in Apex and its dependencies
```

### Installation

The recommended way is to download and decompress an executable from the latest [release](https://github.com/achere/g-force/releases) rather than installing it with `go install`.

### Usage

`apexcov` takes a [package.xml manifest](https://trailhead.salesforce.com/content/learn/modules/package-xml/package-xml-adventure) as input and returns a string of space-separated list of Apex test class names that provide coverage for the included Apex classes and triggers.
This list can then be passed to an [`sf project deploy start -l RunSpecifiedTests`](https://developer.salesforce.com/docs/atlas.en-us.sfdx_cli_reference.meta/sfdx_cli_reference/cli_reference_project_commands_unified.htm#cli_reference_project_deploy_start_unified) command (or `sf project deploy validate ...`) as an argument for the `-t` flag.
Since you can also have [destructive changes separately](https://developer.salesforce.com/docs/atlas.en-us.api_meta.meta/api_meta/meta_deploy_deleting_files.htm), `apexcov` supports parsing multiple .xml files. Provide a comma-separated list of paths to .xml files via the `-packages` flag.

The tool connects to an org that must have the coverage information for the metadata specified in the package.xml file which requires the tests to be run prior to `apexcov`.
In case of insufficient coverage (less than 75% for all code to be deployed or any individual class or trigger), `apexcov` will exit with code 1 and will print the error to the stderr.
Currently, the connection supports only the [Client Credentials Flow](https://help.salesforce.com/s/articleView?id=sf.remoteaccess_oauth_client_credentials_flow.htm&type=5) so you have to have the Connected App set up with [appropriate settings](https://help.salesforce.com/s/articleView?id=sf.connected_app_client_credentials_setup.htm&type=5). Provide the authentication information as a path to a JSON file with the following fields via the `-config` flag:
```json
{
    "apiVersion":   "60.0",
    "baseUrl":      "https://your-domain.my.salesforce.com",
    "clientId":     "CONSUMER_KEY",
    "clientSecret": "CONSUMER_SECRET",
}

```

Tests can be provided using different strategies by passing an appropriate value to the `-strategy` flag:

`MaxCoverage` Maximum coverage
: Outputs all the tests that cover the provided Apex classes and triggers contained in the passed package.xml files

`MaxCoverageWithDeps` Maximum coverage with dependencies
: First calls the Salesforce Metadata Dependency API to collect all Apex classes the classes and triggers in the package.xml files depend on, then request and parse code coverage for both initial classes and their dependencies. Code coverage requirements are skipped for the dependencies as they are not mandatory for the deployment.

### Related

A [package](https://github.com/achere/g-force-sf) that, when installed in an org, can be connected to Gitlab to create a config.json file with authentication information for that org as a CI/CD variable.


