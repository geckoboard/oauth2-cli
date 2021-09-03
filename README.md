[![CircleCI](https://circleci.com/gh/geckoboard/oauth2-cli.svg?style=svg)](https://circleci.com/gh/geckoboard/oauth2-cli)

# oauth2-cli

This is a small command line utility to get an OAuth access token for
three-legged flows where you authorize an application to access your
account, such as [Strava][].

[Strava]: http://strava.github.io/api/partner/v3/oauth/

It is useful for other command line utilities where you need an access token
but don't want to host the application on the web.

## Usage

Install:

    go install github.com/geckoboard/oauth2-cli@latest

Create an API application in the service of your choosing and set the
callback URL to as follows:

    http://127.0.0.1:8080/oauth/callback

Run with all of the necessary arguments, for example:

    $ oauth2-cli \
      -id REDACTED \
      -secret REDACTED \
      -auth https://www.strava.com/oauth/authorize \
      -token https://www.strava.com/oauth/token \
      -scope view_private

You'll then be given a URL to visit from the CLI output, follow that and 
any subsequent instructions.

## Scopes

Multiple scopes can be given by specifying the argument multiple times:

    -scope read \
    -scope write \

Some services are lenient with their interpretation of the OAuth
specification so you will need to specify multiple scopes as a single comma
separated argument:

    -scope write,view_private
