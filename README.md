Generates periodical reports for repositories or organization or
aggregating statistics for current situation in these repos.

## Install

Download binary for your platform from GitHub releases:
https://github.com/g4s8/reporter/releases/latest

Use shell script to get latest release binary (only Linux and MacOSx):
```sh
curl -L https://raw.githubusercontent.com/g4s8/reporter/master/scripts/download.sh | sh
```

On MacOS you can install it using `brew` tool:
```sh
brew tap g4s8/.tap https://github.com/g4s8/.tap
brew install reporter
```

Build from sources:
```sh
git clone https://github.com/g4s8/reporter.git
cd reporter
go build
# target binary will be placed at $PWD/reporter
```

## Usage

Create API token without permissions (to increase GitHub API quota limits).
To use this token with reporter:
 1. Put it to `~/.config/reporter/github_token.txt`
 2. Set it to `GITHUB_TOKEN` environment variable
 3. Use `--token` CLI option

The syntax is: `reporter <action> <source>`
where actions is either `report` or `stats`,
and source is either organization name (for full report over all repositories)
or full repository coordinates (`user/repo`).

To generate daily report run:
```bash
./reporter report artipie
```
To filter pull requests by user use `--author=<username>`,
where `<username>` is either full GitHub username (ignore case) or `me` keyowrd for
current user.
