# GoReadManga
Find, read, maybe.

### Features ✨

- 🚀 **Convenient & Fast**: Quick fetching and searching for manga.
- 🔄 **Resume Where You Left Off**: Easily continue your reading session.
- 🕵️‍♂️ **Browse History**: Access previously viewed material using natively installed `fzf` (or the built-in `fzf` search if not installed).
- 📁 **PDF Storage**: Generated PDFs are stored in a directory on your OS's temp directory (Windows/Android/Linux/Darwin).
- 🖼️ **Image Processing**: Choose between using `jpegli` or the standard JPEG library for encoding/decoding images.
- 📄 **Vertical Image Splitting**: Split tall vertical images into multiple pages without gaps.
- 🌐 **Horizontal Image Splitting**: Split wide horizontal images into multiple pages.
- 📊 **Viewing Statistics**: Get basic statistics on your reading habits.
- 🎨 **Customizable PDF Background**: Change the color of empty space in PDFs (default: black).
- 🔄 **Server Switching**: Easily switch between different content servers.
- 🧹 **Cache Management**: Clear cache easily (it grows fast!).
- 🔍 **Upcoming Features**:
  - Similar title suggestions for easier querying.
  - Command line option to specify output directory.
- 💡 **More to Come**: Stay tuned for additional features as they develop!


![image](https://github.com/user-attachments/assets/0e1792f4-dbc6-4bf0-8217-bb27a97c4cfc)


### Commands in program
![image](https://github.com/user-attachments/assets/1d2030e8-a938-468b-8362-72672903afd3)


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
![image](https://github.com/user-attachments/assets/3f9c5ca2-8cb1-4c7e-86e0-a8bce84df090)

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
