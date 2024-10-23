# GoReadManga
Find, read, maybe.



### Commands in program

| Command | Description |
|---|---|
| `N` | Next chapter |
| `P` | Previous chapter |
| `S` | Select chapter |
| `R` | Reopen current chapter |
| `A` | Search another manga |
| `BH` | Browse history, select to read |
| `ST` | See stats |
| `OD` | Open PDF dir |
| `CS` | Toggle between content server1/2 |
| `D` | Toggle image decoding method [jpegli/normal] |
| `M` | Toggle jpegli encoding mode [jpegli/normal] |
| `WS` | Toggle splitting images wider than page |
| `C` | Clear cache |
| `Q` | Exit |

### Command Line Arguments
**Usage:**

  GoReadManga [Option]


**Options:**

| Option                        | Description                                                |
|-------------------------------|------------------------------------------------------------|
| `-h`, `--help`               | Print this help page                                       |
| `-v`, `--version`            | Print version number                                       |
| `-jp`, `--jpegli`            | Use jpegli to re-encode jpegs                            |
| `-q`, `--quality`            | Set quality to use with jpegli encoding (default: 85)    |
| `-ws`, `--wide-split`        | Split images that are too wide and maximize vertically     |
| `-H`, `--history`            | Show last viewed manga entry in history                   |
| `-bh`, `--browse-history`    | Browse history file, select and read                      |
| `-st`, `--stats`             | Show history statistics                                    |
| `-r`, `--resume`             | Continue from last session                                 |
| `-od`, `--opendir`           | Open pdf directory                                        |
| `-c`, `--cache-size`         | Print cache size (C:/Windows/Temp/.cache/goreadmanga)   |
| `-C`, `--clear-cache`        | Purge cache directory (C:/Windows/Temp/.cache/goreadmanga) |

*Note: The cache directory path is an example; the application will use the OS's temporary directory by default.*


Disclaimer: for personnel and edumucational porpoises only.
