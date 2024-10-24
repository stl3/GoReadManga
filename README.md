# GoReadManga
Find, read, maybe.

### Features ✨

- 🚀 **Convenient & Fast**: Quickly fetch and search for manga with ease.
- 🔄 **Resume Where You Left Off**: Easily continue your reading session.
- 🕵️‍♂️ **Browse History**: Access previously viewed material using natively installed `fzf`, or utilize the built-in `fzf` search if not installed.
- 📁 **PDF Storage**: Generated PDFs are stored in your OS's temp directory (compatible with Windows, Android, Linux, and Darwin).
- 🖼️ **Image Processing**: Choose between `jpegli` or the standard JPEG library for efficient encoding/decoding of images.
- 📄 **Vertical Image Splitting**: Split tall vertical images into multiple pages without any gaps.
- 🌐 **Horizontal Image Splitting**: Split wide horizontal images into multiple pages (maximizes image vertically).
- 📊 **Viewing Statistics**: Get basic statistics on your reading habits.
- 🔄 **Server Switching**: Easily switch between different content servers.
- 🧹 **Cache Management**: Clear cache easily (it can grow quickly!).
- 🔧 **Error Handling**: Cull broken entries in the history JSON file caused by network drops or outages.
- 🗂️ **Comprehensive History Tracking**: Reads stats from all history files (Backups are made when main history json file reaches 5mb).

### 🔍 Upcoming Features:
- 🎯 **Title-Based Recommendations**: Recommender based on title supplied.
- 📂 **Custom Output Directory**: Specify an output directory using the `-o`, `--output-dir` option.
- 🌐 **Proxy Support**: Use a SOCKS5 proxy with the `-ph`, `--proxy-host` option.
- 🎨 **Customizable PDF Background**: Change the color of empty space in PDFs (default: black).
- 🎲 **Randomization Options**: Randomizer or randomize based on genre.

💡 **More to Come**: Stay tuned for additional features as they develop!

![image](https://github.com/user-attachments/assets/0e1792f4-dbc6-4bf0-8217-bb27a97c4cfc)


### Commands in program
![image](https://github.com/user-attachments/assets/1cb7862b-1800-4f92-8c0a-f74be3f9df11)






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
![image](https://github.com/user-attachments/assets/44bd9ada-c4a4-4eca-972f-5c141e5f2747)


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
| `-c`, `--cache-size`         | Print cache size (C:\Users\Administrator\AppData\Local\Temp\.cache\goreadmanga)   |
| `-C`, `--clear-cache`        | Purge cache directory (C:\Users\Administrator\AppData\Local\Temp\.cache\goreadmanga) |
| `-f`, `--fix`        | Remove json entries causing problems (empty chapter_page/chapter_title during network issues) |

*Note: The cache directory path is an example; the application will use the OS's temporary directory by default.*


Disclaimer: for personnel and edumucational porpoises only.
