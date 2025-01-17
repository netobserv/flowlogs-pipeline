# Contributing to flowlogs-pipeline

_See also: [NetObserv projects contribution guide](https://github.com/netobserv/documents/blob/main/CONTRIBUTING.md)_

:fire: A big welcome and thank you for considering contributing to flowlogs-pipeline open source project! :fire:  

We want to make contributing to this project as easy and transparent as possible. 
Please read the following notes to learn how the contribute process works.

## Pull Request Process

1. Follow [pull requests](http://help.github.com/pull-requests/) guidelines.
1. We use git actions to gate pull requests.   
   - Pull Requests need to at least pass `make build` and `make test`.
   - Pull Requests need to be approved by at least one reviewer.  
1. For golang code, follow standard coding convention as described [here](https://go.dev/doc/effective_go).
   - Use `go fmt` and `golangci-lint` to lint and verify code style.
1. for all other contributions, follow existing project coding style (by example).    
1. Connecting an issue to pull Request is always a preference.
1. Always write a clear log message for your commits. One-line messages are fine for small changes, 
   but bigger changes should look like this:
   
```shell
> git commit -m "A brief summary of the commit

A paragraph describing what changed and its impact."
```

## Bugs, enhancements and questions

1. Use git [issues](https://docs.github.com/en/issues/tracking-your-work-with-issues/about-issues) to document bugs, enhancements and questions.
1. Be as clear, concise and simple as possible.
1. Good example can always help to clear things. Please use examples when possible.


## Code of Conduct

### Our Pledge

In the interest of fostering an open and welcoming environment, we as
contributors and maintainers pledge to making participation in our project and
our community a harassment-free experience for everyone, regardless of age, body
size, disability, ethnicity, gender identity and expression, level of experience,
nationality, personal appearance, race, religion, or sexual identity and
orientation.

### Our Standards

Examples of behavior that contributes to creating a positive environment
include:

* Using welcoming and inclusive language
* Being respectful of differing viewpoints and experiences
* Gracefully accepting constructive criticism
* Focusing on what is best for the community
* Showing empathy towards other community members

Examples of unacceptable behavior by participants include:

* The use of sexualized language or imagery and unwelcome sexual attention or
  advances
* Trolling, insulting/derogatory comments, and personal or political attacks
* Public or private harassment
* Publishing others' private information, such as a physical or electronic
  address, without explicit permission
* Other conduct which could reasonably be considered inappropriate in a
  professional setting

### Our Responsibilities

Project maintainers are responsible for clarifying the standards of acceptable
behavior and are expected to take appropriate and fair corrective action in
response to any instances of unacceptable behavior.

Project maintainers have the right and responsibility to remove, edit, or
reject comments, commits, code, wiki edits, issues, and other contributions
that are not aligned to this Code of Conduct, or to ban temporarily or
permanently any contributor for other behaviors that they deem inappropriate,
threatening, offensive, or harmful.

### Scope

This Code of Conduct applies both within project spaces and in public spaces
when an individual is representing the project or its community. Examples of
representing a project or community include using an official project e-mail
address, posting via an official social media account, or acting as an appointed
representative at an online or offline event. Representation of a project may be
further defined and clarified by project maintainers.

### Enforcement

Instances of abusive, harassing, or otherwise unacceptable behavior may be
reported by contacting the project team. All
complaints will be reviewed and investigated and will result in a response that
is deemed necessary and appropriate to the circumstances. The project team is
obligated to maintain confidentiality with regard to the reporter of an incident.
Further details of specific enforcement policies may be posted separately.

Project maintainers who do not follow or enforce the Code of Conduct in good
faith may face temporary or permanent repercussions as determined by other
members of the project's leadership.

### Attribution

This Code of Conduct is adapted from the [Contributor Covenant][homepage], version 1.4,
available at [http://contributor-covenant.org/version/1/4][version]
