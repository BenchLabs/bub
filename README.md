# bub

`bench-hub` -> `bhub` -> `bub` a developer workflow cli tool developed at [bench.co](https://bench.co)

![](https://upload.wikimedia.org/wikipedia/commons/thumb/3/3f/Lil_Bub_2013_%28crop_for_thumb%29.jpg/440px-Lil_Bub_2013_%28crop_for_thumb%29.jpg)

Some examples of things bub tries to help with:

```
COMMANDS:
     setup          Setup bub on your machine.
     update         Update the bub command to the latest release.
     config         Edit your bub config.
     repository, r  Repository related commands.
     manifest, m    Manifest related commands.
     ec2, e         EC2 related related actions. The commands 'bash', 'exec', 'jstack' and 'jmap' will be executed inside the container.
     rds, r         RDS actions.
     route53, 53    R53 actions.
     beanstalk, eb  Elasticbeanstalk actions. If no sub-command specified, lists the environements.
     github, gh     GitHub related commands.
     jira, ji       JIRA related commands.
     workflow, w    Git/GitHub/JIRA workflow commands.
     jenkins, j     Jenkins related commands.
     splunk, s      Splunk related commands.
     confluence, c  Confluence related commands.
     circle         CircleCI related commands.
     help, h        Shows a list of commands or help for one command
```

## Warning / Disclaimer

Everything here is experimental and the commands are subject to, and will,
change. Also, when listing the manifests the list is also incomplete, until each
project is properly updated.


## Installation

### From source

Install prereqs below.

    $ make install

## Setup

To setup bub, you need to have AWS credentials. Run:

    $ bub setup

First you'll be prompted to enter your AWS credentials in the `~/.aws/credentials`. Then bub will create 
the `~/.config/bub/config.yml`. You don't have to edit it unless you want to add some credentials to get 
more features. Adding your Jenkins credentials makes bub super useful.

## Usage

To be expanded, when in doubt, `-h` with any command/sub-command should give you
an idea of what you can do.

    # from any directory
    $ bub # or with --help
    $ bub eb
    $ bub ec2

    # in a repo
    $ bub gh repo
    $ bub gh issues
    # ...

## Prerequisites

    # macOS to use the open commands (you can symlink xdg-open to open on Linux)
    $ brew install golang # tested with 1.8.1 must fix version in future.
    $ brew install graphviz

## Build

    $ make deps
    $ make
    $ bin/bub<your-platform>
