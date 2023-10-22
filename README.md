# tmuxer - Project Tmux Session Manager

tmuxer is a command-line tool written in Go that helps you manage your tmux sessions on a project basis. It simplifies the process of creating, attaching to, and organizing tmux sessions for different projects.


## Features

- Create tmux sessions associated with specific projects.
- Automatically attach to an existing session for a project or create a new one.
- Easily switch between project sessions with `fzf`-like fuzzy finder.

## Installation
### Using go
Install [go](https://go.dev) (Follow Go installation guide [here](https://go.dev/dl/))
```sh
go install github.com/k1ng440/tmuxer@latest
```

Note: Other method of install will be available soon

## Usage

### Configuration
tmux uses a configuration file located at ~/.config/project-tmux/config.yaml to find projects.
Here is an example configuration:

```yaml
base: 
  - ~/code/**/{.git}
  - ~/Projects/**/{.git}
  - ~/Projects/**/{go.mod}
```

### Commands
```bash
tmuxer
```

## Contribution
Contributions to the project are welcome. If you have suggestions, ideas, or improvements, feel free to open issues and pull requests on our GitHub repository.

## License
Project Tmux Session Manager is open-source software licensed under the MIT License. See the [LICENSE](license) file for more details.

