package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif" // Import GIF decode
	"image/jpeg"
	_ "image/jpeg" // Import JPEG decoder
	"image/png"
	_ "image/png" // Import PNG decoder
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/brotli"
	"github.com/olekukonko/tablewriter"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gen2brain/jpegli"
	"github.com/jung-kurt/gofpdf"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/sync/semaphore"
	"golang.org/x/time/rate"
)

const (
	version     = "1.0.0"
	historyFile = "goreadmanga_history.json"
)

type MangaResult struct {
	Title string
	URL   string
}

type Chapter struct {
	Number int
	URL    string
}

type BrowseRecord struct {
	MangaTitle    string    `json:"manga_title"`
	ChapterNumber int       `json:"chapter_number"`
	ChapterPage   string    `json:"chapter_page"`
	ChapterTitle  string    `json:"chapter_title"`
	Timestamp     time.Time `json:"timestamp"`
}

type model struct {
	records []BrowseRecord
	cursor  int
}

var (
	cacheDir         string // Directory to hold files, preferably temp
	currentManga     string
	servers          = []string{"server2", "server1"} // Switch between content servers serving media
	contentServer    string
	isJPMode         bool        // check whether user wants jpegli enabled
	isCCacheMode     bool        // This check is done so we don't print storage size when inside program since it is called in inputControls()
	useFancyDecoding      = true // Flag for toggling decoding method
	jpegliQuality    int  = 85   // Default quality for jpegli encoding
	titleStyle            = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF79C6"))
	titleStyleWithBg      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF79C6")).Background(lipgloss.Color("#00194f"))
	// titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFEB3B"))
	subtitleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD"))
	textStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#F8F8F2"))
	infoStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#ebeb00"))
	highlightStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B"))
	cyanColor      = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FFFF"))
	inputStyle     = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFB86C"))
	versionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFB86C")).
			Background(lipgloss.Color("#282A36")).
			Padding(0, 2) // Adds horizontal padding to the version text
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#8BE9FD")). // Light blue header
			Background(lipgloss.Color("#282A36")).
			Padding(0, 2)
	resultStyle                                        = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#50FA7B")). // Green for results
			Padding(0, 2)
	indexStyle                                        = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF79C6")). // Pink for index numbers
			PaddingRight(1)
	bracketStyle = lipgloss.NewStyle().
		// Foreground(lipgloss.Color("#FF79C6")) // Pink for brackets, no padding
		Foreground(lipgloss.Color("#FF79C6")) // Pink for brackets, no padding
	// chapterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#BD93F9"))
	chapterStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#95fb17"))
	chapterStyleWithBG = lipgloss.NewStyle().Foreground(lipgloss.Color("#95fb17")).Background(lipgloss.Color("#282A36"))
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	debug.SetMaxStack(1000000000)

	checkJPFlag()
	checkCCacheFlag()
	//////////////////////////////////////////////////////////
	// Only for my personal config
	// Get the hostname of the machine
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Println("Error retrieving hostname:", err)
		return
	}
	// fmt.Println("Hostname is:", hostname) // Just for debugging
	// Check if the OS is Windows and the hostname matches "7950X3D" or "DEMONSEED-W10"
	if runtime.GOOS == "windows" && (hostname == "7950x3d" || hostname == "DemonSeed-W10") {
		// Check if the T: drive is available
		if _, err := os.Stat("T:/"); os.IsNotExist(err) {
			// Fallback to tempDir if T: drive is not available
			fmt.Println("T: Drive does not exist:", err)
			tempDir := os.TempDir()
			cacheDir = filepath.Join(tempDir, ".cache", "goreadmanga")
		} else {
			cacheDir = "T:/.cache/goreadmanga"
		}
	} else {
		// Fallback to tempDir for non-matching hostnames or non-Windows OS
		tempDir := os.TempDir()
		cacheDir = filepath.Join(tempDir, ".cache", "goreadmanga")
	}
	/////////////////////////////////////////////////////////////
	// When distributing public
	// // tempDir := os.TempDir()
	// // cacheDir = filepath.Join(tempDir, ".cache", "goreadmanga")
	////////////////////////////////////////////////////////////
}

func main() {
	setupSignalHandling()

	if len(os.Args) > 1 {
		handleArguments(os.Args[1:])
	} else {
		searchAndReadManga()
	}
}

func setupSignalHandling() {
	// If user interrupts program quit like a graceful swan maybe
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\n\nProgram Interrupted.")
		/////////////////////////////////////////
		// Enable to clear cache on interrupt
		/////////////////////////////////////////
		// os.RemoveAll(filepath.Join(cacheDir, getModMangaTitle(currentManga)))
		// fmt.Println("\n💥💥 " + cyanColor.Render("Nuking cache...") + " 💥💥")
		/////////////////////////////////////////
		os.Exit(0)
	}()
}

func handleArguments(args []string) {
	switch args[0] {
	case "-h", "--help":
		showHelp()
	case "-v", "--version":
		showVersion()
	case "-H", "--history":
		showHistory()
	case "-hb", "--historyb":
		lookHistory()
	case "-tr", "--historytr":
		showHistoryWithTablewriter()
	case "-r", "--resume":
		openLastSession(historyFile)
	case "-c", "--cache-size":
		showCacheSize()
	case "-C", "--clear-cache":
		clearCache()
	default:
		searchAndReadManga()
	}
}

func showHelp() {
	title := titleStyle.Render("goreadmanga " + version + " (github.com/stl3/goreadmanga)")
	subtitle := subtitleStyle.Render("App for finding manga via the terminal")
	usage := highlightStyle.Render("Usage:")
	options := highlightStyle.Render("Options:")
	optionText := textStyle.Render(`
  -h, --help           Print this help page
  -v, --version        Print version number
  -jp --jpegli         Use jpegli to re-encode jpegs
  -q --quality		 Set quality to use with jpegli encoding (default: 85)
  -H, --history   	 Show last viewed manga entry in history
  -hb, --historyb      Display history using bubbletea
  -tr, --historytr     Display history using tablewriter
  -r, --resume   	  Continue from last session
  -c, --cache-size     Print cache size (` + cacheDir + `)
  -C, --clear-cache    Purge cache dir (` + cacheDir + `)
`)

	fmt.Printf(`
%s
%s

%s

  GoReadManga [Option]

%s
%s
`, title, subtitle, usage, options, optionText)
}

func showVersion() {
	versionText := versionStyle.Render("Version: " + version)
	fmt.Println(versionText)
}

func showCacheSize() {
	size, err := getDirSize(cacheDir)
	if err != nil {
		////////// Debug Message //////////
		// fmt.Printf("Error getting cache size or doesn't exist: %v\n", err)
		///////////////////////////////////
		return
	}
	fmt.Printf("Cache size: %s (%s)\n", formatSize(size), cacheDir)
}

func clearCache() {
	if !isCCacheMode { // Unlikely case, but if -C or --clear-cache ran with program we prevent it from showing duplicate cache size in menu
		showCacheSize()
	}

	if promptYesNo("Proceed with clearing the cache?") {
		err := os.RemoveAll(cacheDir)
		if err != nil {
			////////// Debug Message //////////
			// fmt.Printf("Error clearing cache: %v\n", err)
			///////////////////////////////////
		} else {
			fmt.Println("💥💥 " + cyanColor.Render("Cache successfully cleared") + " 💥💥")
		}
	}
}

// Update the searchAndReadManga function to set the currentManga
func searchAndReadManga() {
	mangaTitleInput := promptUser("Search manga:")
	fmt.Printf("Searching for '%s'...\n", mangaTitleInput)

	searchResults := scrapeMangaList(mangaTitleInput)
	if len(searchResults) == 0 {
		fmt.Println("No search results found")
		searchAndReadManga()
		return
	}

	displaySearchResults(searchResults)
	selectedManga := selectManga(searchResults)
	currentManga = selectedManga.Title

	chapters := scrapeChapterList(selectedManga.URL)
	if len(chapters) == 0 {
		fmt.Println("No chapters found. Exiting...")
		os.Exit(1)
	}

	selectedChapter := selectChapter(chapters)
	openChapter(selectedManga, selectedChapter)
	inputControls(selectedManga, chapters, selectedChapter)
}

func scrapeMangaList(query string) []MangaResult {
	baseURL := fmt.Sprintf("https://manganato.com/search/story/%s", strings.ReplaceAll(query, " ", "_"))
	doc, err := fetchDocument(baseURL)
	if err != nil {
		fmt.Printf("Error fetching search results: %v\n", err)
		return nil
	}

	// Find total number of pages
	lastPage := 1
	doc.Find(".panel-page-number .page-last").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			// Extract the page number from the href
			var num int
			fmt.Sscanf(href, baseURL+"?page=%d", &num)
			if num > lastPage {
				lastPage = num
			}
		}
	})

	// Limit to maximum 5 pages
	maxPages := 5
	if lastPage < maxPages {
		maxPages = lastPage
	}

	var results []MangaResult

	// Create a rate limiter that allows 1 request per second
	limiter := rate.NewLimiter(1, 1)

	// Scrape results from the first page up to maxPages
	for page := 1; page <= maxPages; page++ {
		// Wait for the next available slot
		limiter.Wait(context.Background())

		url := baseURL
		if page > 1 {
			url = fmt.Sprintf("%s?page=%d", baseURL, page)
		}

		doc, err := fetchDocument(url)
		if err != nil {
			fmt.Printf("Error fetching page %d: %v\n", page, err)
			continue
		}

		doc.Find(".panel-search-story .item-right").Each(func(i int, s *goquery.Selection) {
			title := s.Find("h3 a").Text()
			href, _ := s.Find("h3 a").Attr("href")
			results = append(results, MangaResult{Title: title, URL: href})
		})
	}

	return results
}

func displaySearchResults(results []MangaResult) {
	header := headerStyle.Render(fmt.Sprintf("Found %d result(s):", len(results)))
	fmt.Println(header)

	for i, result := range results {
		index := indexStyle.Render(fmt.Sprintf("[%d]", i+1))
		resultText := resultStyle.Render(result.Title)
		fmt.Printf("%s %s\n", index, resultText)
	}
	fmt.Println()
}

func selectManga(results []MangaResult) MangaResult {
	if len(results) == 1 {
		fmt.Printf("Selected '%s'\n", results[0].Title)
		return results[0]
	}

	for {
		selection := promptUser(fmt.Sprintf("Select manga [1-%d]:", len(results)))
		index, err := strconv.Atoi(selection)
		if err != nil || index < 1 || index > len(results) {
			fmt.Println("Invalid selection")
			continue
		}
		return results[index-1]
	}
}

func scrapeChapterList(mangaURL string) []Chapter {
	doc, err := fetchDocument(mangaURL)
	if err != nil {
		fmt.Printf("Error fetching chapter list: %v\n", err)
		return nil
	}

	var chapters []Chapter
	doc.Find(".row-content-chapter li").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Find("a").Attr("href")
		// Append chapters normally
		chapters = append(chapters, Chapter{URL: href})
	})

	// Reverse the order of chapters
	for i := len(chapters)/2 - 1; i >= 0; i-- {
		opp := len(chapters) - 1 - i
		chapters[i], chapters[opp] = chapters[opp], chapters[i]
	}

	// Assign numbers after reversing
	for i := range chapters {
		chapters[i].Number = i + 1 // Set correct numbering after reversing
	}

	return chapters
}

func selectChapter(chapters []Chapter) Chapter {
	if len(chapters) == 1 {
		fmt.Println("Selected first chapter")
		return chapters[0]
	}

	for {
		selection := promptUser(fmt.Sprintf("Select chapter [1-%d]:", len(chapters)))
		index, err := strconv.Atoi(selection)
		if err != nil || index < 1 || index > len(chapters) {
			fmt.Println("Invalid selection")
			continue
		}
		return chapters[index-1]
	}
}

func openChapter(manga MangaResult, chapter Chapter) {
	images, chapterTitle := scrapeChapterImages(chapter.URL)
	pdfPath := downloadAndConvertToPDF(manga, chapter, images, chapterTitle)
	openPDF(pdfPath)
}

func sanitizeFilename(name string) string {
	// Replace illegal characters with an underscore
	// Windows illegal characters: \ / : * ? " < > |
	illegalChars := regexp.MustCompile(`[<>:"/\\|?*]`)
	return illegalChars.ReplaceAllString(name, "_")
}

func scrapeChapterImages(chapterURL string) ([]string, string) {
	var images []string
	var currentServer string
	var doc *goquery.Document
	var err error

	for _, server := range servers {
		////////// Debug Message //////////
		// fmt.Printf("Trying server: %s\n", server)
		///////////////////////////////////
		currentServer = getImageServer(server, chapterURL)
		if currentServer == "" {
			fmt.Printf("Failed to get image server for %s\n", server)
			continue
		}

		doc, err = fetchDocument(chapterURL)
		if err != nil {
			fmt.Printf("Error fetching chapter images: %v\n", err)
			continue
		}

		images = []string{}
		doc.Find(".container-chapter-reader img").Each(func(i int, s *goquery.Selection) {
			src, exists := s.Attr("src")
			if exists {
				parsedURL, err := url.Parse(src)
				if err == nil {
					parsedURL.Host = currentServer
					images = append(images, parsedURL.String())
				}
			}
		})

		if len(images) > 0 {
			break
		}
	}

	if len(images) == 0 {
		fmt.Println("Failed to find any images")
		return nil, ""
	}

	chapterTitle := doc.Find(".panel-chapter-info-top h1").Text()
	chapterTitle = sanitizeFilename(chapterTitle)
	fmt.Printf("Found %d image URLs\n", len(images))
	return images, chapterTitle
}

func downloadAndConvertToPDF(manga MangaResult, chapter Chapter, imageURLs []string, chapterTitle string) string {
	// Create a record for the current manga and chapter
	record := BrowseRecord{
		MangaTitle:    manga.Title,
		ChapterNumber: chapter.Number,
		ChapterTitle:  chapterTitle,
		ChapterPage:   chapter.URL,
	}

	// Record the browsing history
	if err := recordBrowseHistory(historyFile, record); err != nil {
		fmt.Printf("Error recording history: %v\n", err)
	}

	mangaDir := filepath.Join(cacheDir, getModMangaTitle(manga.Title))
	////////// Debug Message //////////
	// fmt.Printf("manga title: %s\n", manga.Title)
	///////////////////////////////////

	// Format the PDF filename with the chapter number and title
	// pdfFilename := fmt.Sprintf("chapter_%d-%s.pdf", chapter.Number, chapterTitle)
	pdfFilename := fmt.Sprintf("%s.pdf", chapterTitle)
	pdfPath := filepath.Join(mangaDir, pdfFilename)

	// Return if PDF already exists
	if _, err := os.Stat(pdfPath); err == nil {
		return pdfPath
	}

	// Create directories for chapter and images
	chapterDir := filepath.Join(mangaDir, fmt.Sprintf("chapter_%d", chapter.Number))
	os.MkdirAll(chapterDir, os.ModePerm)

	fmt.Println("Downloading images...")

	// Preallocate a slice to store the image paths in order
	imagePaths := make([]string, len(imageURLs))
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Semaphore to limit concurrent downloads, default 5
	// Seems to crash now when higher than 1
	// 1 for jpegli
	// // maxConcurrentDownloads := int64(1)
	maxConcurrentDownloads := int64(1)
	/////////////////////////
	// Enable for testing [no bueno right now, it crashes almost always and it seems fast enough by default]
	// Adjust time.sleep below instead
	// if isJPMode {
	// 	maxConcurrentDownloads = int64(2)
	// }
	/////////////////////////
	sem := semaphore.NewWeighted(maxConcurrentDownloads)

	bar := progressbar.New(len(imageURLs)) // Initialize the progress bar

	for i, url := range imageURLs {
		wg.Add(1)
		// Start go routine for each download
		go func(i int, url string) {
			defer wg.Done()
			if err := sem.Acquire(context.Background(), 1); err != nil {
				fmt.Printf("Failed to acquire semaphore: %v\n", err)
				return
			}
			defer sem.Release(1)
			// This part only added for prevention of being rate limited
			// but still testing
			// < 100 fast > 500 slow
			time.Sleep(100 * time.Millisecond)
			////////// Debug Message //////////
			// print(url)
			///////////////////////////////////
			imagePath := filepath.Join(chapterDir, fmt.Sprintf("%d.jpg", i+1))

			if !isJPMode {
				fmt.Printf("\rDownloading image %d from: %s\r\n", i+1, url)
				bar.Add(1)
			}

			err := downloadFile(url, imagePath)
			if err != nil {
				fmt.Printf("Error downloading image %d: %v\n", i+1, err)
				return
			}

			if verifyImage(imagePath) {
				mu.Lock()
				imagePaths[i] = imagePath
				mu.Unlock()
			} else {
				fmt.Printf("Invalid image file: %s\n", imagePath)
				// Not sure if this is needed
				// os.Remove(imagePath)
			}

			if isJPMode {
				bar.Add(1)
			}
		}(i, url)

		///////////////////////////////////////////////////////////////////////////////
		////////// Debug Message //////////
		// Monitor number of active goroutines
		// fmt.Printf("Active goroutines: %d\n", runtime.NumGoroutine())
		///////////////////////////////////

	}

	// Wait for all downloads to finish
	wg.Wait()
	// Let's go crazy with garbage collection
	runtime.GC()
	// Release semaphore, if you wantses it
	// sem.Release(1)

	// This may not be needed, maybe. Just keep it for now.
	// Remove any empty entries in imagePaths (if some downloads failed)
	finalImagePaths := []string{}
	for _, path := range imagePaths {
		if path != "" {
			finalImagePaths = append(finalImagePaths, path)
		}
	}

	// If no valid images were downloaded, return empty result
	if len(finalImagePaths) == 0 {
		fmt.Println("No valid images downloaded. Unable to create PDF.")
		return ""
	}

	fmt.Println("\nConverting images to PDF...")
	err := createPDFFromImages(finalImagePaths, pdfPath)
	if err != nil {
		fmt.Printf("Error creating PDF: %v\n", err)
		return ""
	}
	runtime.GC()
	// Clean up the chapter directory after PDF creation
	os.RemoveAll(chapterDir)
	runtime.GC()
	return pdfPath

}

func createPDFFromImages(imagePaths []string, outputPath string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	for _, imagePath := range imagePaths {
		pdf.AddPage()

		// Get image dimensions
		file, _ := os.Open(imagePath)
		image, _, err := image.DecodeConfig(file)
		file.Close()
		if err != nil {
			return fmt.Errorf("error decoding image %s: %v", imagePath, err)
		}

		// Calculate scaling factors
		pageWidth, pageHeight := pdf.GetPageSize()
		scaleX := pageWidth / float64(image.Width)
		scaleY := pageHeight / float64(image.Height)
		scale := math.Min(scaleX, scaleY)

		width := float64(image.Width) * scale
		height := float64(image.Height) * scale

		// Center the image on the page
		x := (pageWidth - width) / 2
		y := (pageHeight - height) / 2

		pdf.Image(imagePath, x, y, width, height, false, "", 0, "")
	}
	return pdf.OutputFileAndClose(outputPath)
}

func openPDF(pdfPath string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "android":
		// Use an Intent to open the PDF file in Termux
		cmd = exec.Command("termux-open", pdfPath)

	case "darwin": // macOS
		cmd = exec.Command("open", pdfPath)

	case "windows":
		// Check if SumatraPDF is available
		if _, err := exec.LookPath("SumatraPDF.exe"); err == nil {
			cmd = exec.Command("SumatraPDF.exe", "-view", "continuous single page", "-zoom", "fit width", pdfPath)
		} else {
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", pdfPath) // Default PDF reader on Windows
		}

	case "linux":
		// Use xdg-open to open the PDF with the default viewer on Linux
		cmd = exec.Command("xdg-open", pdfPath)

	default:
		fmt.Println("Unsupported OS")
		return
	}

	err := cmd.Start()
	if err != nil {
		fmt.Printf("Error opening PDF: %v\n", err)
	}
}

// // func inputControls(manga MangaResult, chapters []Chapter, currentChapter Chapter) {
// // 	// Function to fetch and update the chapter title
// // 	updateChapterInfo := func(currentChapter Chapter) (string, string) {
// // 		// Regular expression to match 'chapter-' followed by digits
// // 		re := regexp.MustCompile(`(chapter-)\d+$`)
// // 		// Convert the chapter number to a string
// // 		newChapterNumber := strconv.Itoa(currentChapter.Number)
// // 		// Replace the chapter number in the URL
// // 		newCurrentChapterURL := re.ReplaceAllString(currentChapter.URL, "${1}"+newChapterNumber)

// // 		// Fetch the new chapter document
// // 		doc, err := fetchDocument(newCurrentChapterURL)
// // 		if err != nil {
// // 			fmt.Printf("Error fetching chapter images: %v\n", err)
// // 			return "", ""
// // 		}

// // 		// Get the updated chapter title
// // 		chapterTitle := doc.Find(".panel-chapter-info-top h1").Text()
// // 		return newCurrentChapterURL, chapterTitle
// // 	}

// // 	// Initial chapter info
// // 	// currentChapter.URL, chapterTitle := updateChapterInfo(currentChapter)
// // 	newURL, chapterTitle := updateChapterInfo(currentChapter)
// // 	currentChapter.URL = newURL // Explicitly assign the new URL to the struct field

// // 	for {
// // 		/////////////////////////////////////////////
// // 		// My emoji no color on vscodium (Segoe UI Emoji font issue)
// // 		// I had fixed this once, but now it's back
// // 		// Something to do with:
// // 		// [HKEY_CURRENT_USER\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Fonts]
// // 		// [HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Fonts]
// // 		// [HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows NT\CurrentVersion\FontSubstitutes]
// // 		// so I am keeping this here for when it does work
// // 		// fmt.Println(
// // 		// 	bracketStyle.Render("[") +
// // 		// 		chapterStyleWithBG.Render("Chapter") +
// // 		// 		highlightStyle.Render(fmt.Sprintf(" %d/%d", currentChapter.Number, len(chapters))) +
// // 		// 		bracketStyle.Render("] ") + "▄︻デ══━一 🌟💥 " +
// // 		// 		titleStyleWithBg.Render(chapterTitle) + " 💥🌟",
// // 		// )
// // 		/////////////////////////////////////////////
// // 		fmt.Println(
// // 			bracketStyle.Render("[") +
// // 				chapterStyleWithBG.Render("Chapter") +
// // 				highlightStyle.Render(fmt.Sprintf(" %d/%d", currentChapter.Number, len(chapters))) +
// // 				bracketStyle.Render("] ") +
// // 				titleStyleWithBg.Render("▄︻デ══━一 🌟💥 ", chapterTitle, " 💥🌟"),
// // 		)

// // 		fmt.Println(bracketStyle.Render("[") + highlightStyle.Render("N") + bracketStyle.Render("]") + textStyle.Render(" Next chapter"))
// // 		fmt.Println(bracketStyle.Render("[") + highlightStyle.Render("P") + bracketStyle.Render("]") + textStyle.Render(" Previous chapter"))
// // 		fmt.Println(bracketStyle.Render("[") + highlightStyle.Render("S") + bracketStyle.Render("]") + textStyle.Render(" Select chapter"))
// // 		fmt.Println(bracketStyle.Render("[") + highlightStyle.Render("R") + bracketStyle.Render("]") + textStyle.Render(" Reopen current chapter"))
// // 		fmt.Println(bracketStyle.Render("[") + highlightStyle.Render("A") + bracketStyle.Render("]") + textStyle.Render(" Search another manga"))
// // 		fmt.Println(bracketStyle.Render("[") + highlightStyle.Render("CS") + bracketStyle.Render("]") + textStyle.Render(" Toggle between content server1/2"))
// // 		fmt.Println(bracketStyle.Render("[") + highlightStyle.Render("D") + bracketStyle.Render("]") + textStyle.Render(" Toggle image decoding method [jpegli/normal]"))
// // 		fmt.Println(bracketStyle.Render("[") + highlightStyle.Render("M") + bracketStyle.Render("]") + textStyle.Render(" Toggle jpegli encoding mode [jpegli/normal]"))
// // 		fmt.Println(bracketStyle.Render("[") + highlightStyle.Render("C") + bracketStyle.Render("]") + textStyle.Render(" Clear cache"))
// // 		fmt.Println(bracketStyle.Render("[") + highlightStyle.Render("Q") + bracketStyle.Render("]") + textStyle.Render(" Exit"))
// // 		showCacheSize()
// // 		choice := strings.ToLower(promptUser(textStyle.Render("Enter input:")))

// // 		switch choice {
// // 		case "n":
// // 			if currentChapter.Number < len(chapters) {
// // 				// Move directly to the next chapter
// // 				currentChapter = chapters[currentChapter.Number] // Use currentChapter.Number directly to get the next chapter
// // 				currentChapter.URL, chapterTitle = updateChapterInfo(currentChapter)
// // 				checkIfPDFExist(manga, chapterTitle, cacheDir, currentChapter)
// // 			}
// // 		case "p":
// // 			if currentChapter.Number > 1 {
// // 				// Move directly to the previous chapter
// // 				currentChapter = chapters[currentChapter.Number-2] // Use currentChapter.Number-2 to get the previous chapter
// // 				currentChapter.URL, chapterTitle = updateChapterInfo(currentChapter)
// // 				checkIfPDFExist(manga, chapterTitle, cacheDir, currentChapter)
// // 			}
// // 		case "s":
// // 			currentChapter = selectChapter(chapters)
// // 			currentChapter.URL, chapterTitle = updateChapterInfo(currentChapter) // Update title and URL
// // 			checkIfPDFExist(manga, chapterTitle, cacheDir, currentChapter)
// // 		case "r":
// // 			checkIfPDFExist(manga, chapterTitle, cacheDir, currentChapter)
// // 		case "a":
// // 			searchAndReadManga()
// // 			return
// // 		case "cs":
// // 			changeServerOrder()
// // 		case "d":
// // 			toggleDecodingMethod()
// // 		case "m":
// // 			isJPMode = !isJPMode
// // 			if isJPMode {
// // 				fmt.Println("✔️✔️ ⚡⚡ " + indexStyle.Render("jpegli encoding active") + " ⚡⚡ ✔️✔️") // I wish I could see this in color on my damn ide
// // 			} else {
// // 				fmt.Println("❌❌ " + indexStyle.Render("jpegli encoding deactivated") + " ❌❌") // Why don't you show color anymore?
// // 			}
// // 		case "c":
// // 			clearCache()
// // 		case "q":
// // 			os.Exit(0)
// // 		default:
// // 			fmt.Println(subtitleStyle.Render("Invalid input, please try again."))
// // 		}
// // 	}
// // }

func inputControls(manga MangaResult, chapters []Chapter, currentChapter Chapter) {
	// Function to fetch and update the chapter title
	updateChapterInfo := func(currentChapter Chapter) (string, string) {
		re := regexp.MustCompile(`(chapter-)\d+$`)
		newChapterNumber := strconv.Itoa(currentChapter.Number)
		newCurrentChapterURL := re.ReplaceAllString(currentChapter.URL, "${1}"+newChapterNumber)

		doc, err := fetchDocument(newCurrentChapterURL)
		if err != nil {
			fmt.Printf("Error fetching chapter images: %v\n", err)
			return "", ""
		}

		chapterTitle := doc.Find(".panel-chapter-info-top h1").Text()
		return newCurrentChapterURL, chapterTitle
	}

	// Function to display chapter menu
	displayMenu := func(chapterTitle string, currentChapterNumber int, totalChapters int) {
		fmt.Println(
			bracketStyle.Render("[") +
				chapterStyleWithBG.Render("Chapter") +
				highlightStyle.Render(fmt.Sprintf(" %d/%d", currentChapterNumber, totalChapters)) +
				bracketStyle.Render("] ") +
				titleStyleWithBg.Render("▄︻デ══━一 🌟💥 ", chapterTitle, " 💥🌟"),
		)
		fmt.Println(bracketStyle.Render("[") + highlightStyle.Render("N") + bracketStyle.Render("]") + textStyle.Render(" Next chapter"))
		fmt.Println(bracketStyle.Render("[") + highlightStyle.Render("P") + bracketStyle.Render("]") + textStyle.Render(" Previous chapter"))
		fmt.Println(bracketStyle.Render("[") + highlightStyle.Render("S") + bracketStyle.Render("]") + textStyle.Render(" Select chapter"))
		fmt.Println(bracketStyle.Render("[") + highlightStyle.Render("R") + bracketStyle.Render("]") + textStyle.Render(" Reopen current chapter"))
		fmt.Println(bracketStyle.Render("[") + highlightStyle.Render("A") + bracketStyle.Render("]") + textStyle.Render(" Search another manga"))
		fmt.Println(bracketStyle.Render("[") + highlightStyle.Render("CS") + bracketStyle.Render("]") + textStyle.Render(" Toggle between content server1/2"))
		fmt.Println(bracketStyle.Render("[") + highlightStyle.Render("D") + bracketStyle.Render("]") + textStyle.Render(" Toggle image decoding method [jpegli/normal]"))
		fmt.Println(bracketStyle.Render("[") + highlightStyle.Render("M") + bracketStyle.Render("]") + textStyle.Render(" Toggle jpegli encoding mode [jpegli/normal]"))
		fmt.Println(bracketStyle.Render("[") + highlightStyle.Render("C") + bracketStyle.Render("]") + textStyle.Render(" Clear cache"))
		fmt.Println(bracketStyle.Render("[") + highlightStyle.Render("Q") + bracketStyle.Render("]") + textStyle.Render(" Exit"))
		showCacheSize()
	}

	// Function to handle chapter navigation
	handleChapterNavigation := func(choice string, currentChapter *Chapter, chapterTitle *string) {
		switch choice {
		case "n":
			if currentChapter.Number < len(chapters) {
				*currentChapter = chapters[currentChapter.Number]
				currentChapter.URL, *chapterTitle = updateChapterInfo(*currentChapter)
				checkIfPDFExist(manga, *chapterTitle, cacheDir, *currentChapter)
			}
		case "p":
			if currentChapter.Number > 1 {
				*currentChapter = chapters[currentChapter.Number-2]
				currentChapter.URL, *chapterTitle = updateChapterInfo(*currentChapter)
				checkIfPDFExist(manga, *chapterTitle, cacheDir, *currentChapter)
			}
		case "s":
			*currentChapter = selectChapter(chapters)
			currentChapter.URL, *chapterTitle = updateChapterInfo(*currentChapter)
			checkIfPDFExist(manga, *chapterTitle, cacheDir, *currentChapter)
		case "r":
			checkIfPDFExist(manga, *chapterTitle, cacheDir, *currentChapter)
		case "a":
			searchAndReadManga()
			return
		case "cs":
			changeServerOrder()
		case "d":
			toggleDecodingMethod()
		case "m":
			isJPMode = !isJPMode
			displayEncodingStatus()
		case "c":
			clearCache()
		case "q":
			os.Exit(0)
		default:
			fmt.Println(subtitleStyle.Render("Invalid input, please try again."))
		}
	}

	// Main input loop
	newURL, chapterTitle := updateChapterInfo(currentChapter)
	currentChapter.URL = newURL

	for {
		displayMenu(chapterTitle, currentChapter.Number, len(chapters))
		choice := strings.ToLower(promptUser(textStyle.Render("Enter input:")))
		handleChapterNavigation(choice, &currentChapter, &chapterTitle)
	}
}

// Function to display jpegli mode
func displayEncodingStatus() {
	if isJPMode {
		fmt.Println("✔️✔️ ⚡⚡ " + indexStyle.Render("jpegli encoding active") + " ⚡⚡ ✔️✔️")
	} else {
		fmt.Println("❌❌ " + indexStyle.Render("jpegli encoding deactivated") + " ❌❌")
	}
}

func checkIfPDFExist(manga MangaResult, chapterTitle string, cacheDir string, currentChapter Chapter) {
	mangaDir := filepath.Join(cacheDir, getModMangaTitle(manga.Title))
	chapterTitle = sanitizeFilename(chapterTitle)
	pdfFilename := fmt.Sprintf("%s.pdf", chapterTitle)
	pdfPath := filepath.Join(mangaDir, pdfFilename)

	// Return if PDF already exists
	if _, err := os.Stat(pdfPath); err == nil {
		// fmt.Printf(infoStyle.Render("PDF already exists: %s\n"), pdfPath)
		openPDF(pdfPath)
	} else {
		fmt.Printf(infoStyle.Render("PDF doesn't exist: %s\n", pdfPath))
		openChapter(manga, currentChapter)
	}
}

// Function to toggle the decoding method
func toggleDecodingMethod() {
	useFancyDecoding = !useFancyDecoding // Toggle the flag
	if useFancyDecoding {
		fmt.Println(infoStyle.Render("Using fancy decoding options."))
	} else {
		fmt.Println(infoStyle.Render("Using standard decoding."))
	}
}

// Function to change the server order based on a switch case
func changeServerOrder() {
	if servers[0] == "server1" {
		// Toggle the order
		servers = []string{"server2", "server1"}
		fmt.Println(infoStyle.Render("Switched server2"))
	} else {
		// Toggle back to the original order
		servers = []string{"server1", "server2"}
		fmt.Println(infoStyle.Render("Switched server1"))
	}
	// fmt.Println("Switched server order to:", servers)
}

func getModMangaTitle(title string) string {
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == '\'' || r == '/' || r == ':' {
			return '_'
		}
		return r
	}, title)
}

func promptUser(prompt string) string {
	fmt.Print(prompt + " ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func promptYesNo(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [Y/n]: ", prompt)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(response)

	if response == "" {
		return true // Default to yes
	}

	return strings.ToLower(response) == "y"
}

func fetchDocument(url string) (*goquery.Document, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// Add custom headers to the request
	resp.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:128.0) Gecko/20100101 Firefox/128.0")
	resp.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/jxl,image/webp,image/png,image/svg+xml,*/*;q=0.8")
	resp.Header.Set("Accept-Language", "en-US,en;q=0.5")
	resp.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")
	resp.Header.Set("DNT", "1")
	resp.Header.Set("Sec-GPC", "1")
	resp.Header.Set("Referer", url)
	resp.Header.Set("Upgrade-Insecure-Requests", "1")
	resp.Header.Set("Sec-Fetch-Dest", "document")
	resp.Header.Set("Sec-Fetch-Mode", "navigate")
	resp.Header.Set("Sec-Fetch-Site", "same-origin")
	resp.Header.Set("Sec-Fetch-User", "?1")
	resp.Header.Set("TE", "trailers")
	// Add headers to mimic a browser request
	cookie := "ci_session=l9sOULc9Sg3kZ38AeenWTCUtsyQr%%2F77Ex1iIKA%%2Fi39Yy%%2ByETwYCGWoHclLUoOkXmT2vgBvTTXr4G7SKWNlm0Bzm6SSBfkTrbL8dWr20pw1MvSNCNFAP7zjrUqxCC4U6ScC%%2BstNLxgmRWS7Cc10HpZVC0IZpyXNo%%2BTkNAY7D4MNvh%%2FbjYKb68VUgWpoTv9cbFGhOnLVr3dD8SsZa9tLr1H4zPCR%%2F7GzDuqEi4rVN46uz%%2BW7omB7Zfd8Bup6k8Cq9GAW8u3FrNHtZ54eoZz7Yy1uJLzNqTmijUjvXFFFaXOLOlYe6qvzHrH7DTrVgH%%2B98WhGISzpQn367ZZr4Vkr0BYHxMF3ys3%%2BZyswyetYJMJXKDoagXLGrtGHPa26VOtfW9J3s1FdpSbdBnA6qCPAietqtIPmJbh9wy0nGA2ULHrxQ%%3D2f4bd2e67bd41b13e7f45347ad166d195ee68609; content_lazyload=on; content_server=server2; panel-fb-comment=fb-comment-title-show"
	resp.Header.Set("Cookie", cookie)
	resp.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	resp.Header.Set("Referer", url)
	return goquery.NewDocumentFromReader(resp.Body)
}

func getImageServer(contentServer, chapterURL string) string {
	client := &http.Client{}

	urlServer := fmt.Sprintf("https://chapmanganato.to/content_server_s%s", strings.TrimPrefix(contentServer, "server"))
	// fmt.Printf("Requesting URL: %s\n", urlServer)

	req, err := http.NewRequest("GET", urlServer, nil)
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return ""
	}

	// Set headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:128.0) Gecko/20100101 Firefox/128.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/jxl,image/webp,image/png,image/svg+xml,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("DNT", "1")
	req.Header.Set("Sec-GPC", "1")
	req.Header.Set("Referer", chapterURL)
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Priority", "u=0, i")
	req.Header.Set("TE", "trailers")
	req.Header.Set("Authority", "chapmanganato.to")

	// fmt.Printf("content server is: %s\n", contentServer) // For Debug
	var contentServerPath string

	if contentServer == "server1" {
		contentServerPath = "server2"
		// fmt.Printf("content server path is: %s\n", contentServerPath) // For debug
	} else if contentServer == "server2" {
		contentServerPath = "server1"
		// fmt.Printf("content server path is: %s\n", contentServerPath) // For debug
	}

	// fmt.Printf("content server is: %s\n", contentServer)
	// contentServerPath := "server1"
	// if contentServer == "server1" {
	// 	contentServerPath = "server2"
	// 	fmt.Printf("content server path is: %s\n", contentServerPath)
	// }

	// Set cookies
	cookies := []*http.Cookie{
		{Name: "ci_session", Value: "incw9QGKieHVVuooNkSm5vJS9ZUPuJmEVC7tNT2TgCUJYinu0EDzlEulbzmnRN98UC0KqSrN15gDmq6nHXSTe9kmAakV5fKtJ0xFRrNhRr5Xo%2FKVRHMrVGOADvScxP3G2KR1Z3XbMgU1CWw17DG01pElScdmS6riB9vbPfh8B2Euzj9rwfSodgnYDuE3Wqs46Pp0T8Odo%2FpLx5L%2FU4j0QJaiCmE6Yu%2FSoVy%2FBWg%2BWlDEJVLC3OrHlAnmG1b2DPlwTvgmiOHSgk27pl63zST7ppQusjKoy9tdNgHpQaoaejVyRNwXJv%2Bw8qMf%2B5j9hYlzs1m3nqSBwLx%2BIXoux8Q%2FF1Z6Y%2BzcbjdKJBMWeFeKPLHr%2F7DGACEsWTnsGw%2B%2BGycDQeJQjh8KmPx9nRIX9EiNKEyurpjADekCYmznE3JAVMq9NKkXSa72qJZ8gS0OYl12hUhoRMet1MFe1Q5RlM%2FB%2FA%3D%3Dedb59e6619acef07aacb5bbbaefbd98ae564d573", Path: "/", Domain: "chapmanganato.to"},
		{Name: "panel-fb-comment", Value: "fb-comment-title-show", Path: "/", Domain: "chapmanganato.to"},
		{Name: "content_lazyload", Value: "off", Path: "/", Domain: "chapmanganato.to"},
		{Name: "content_server", Value: contentServerPath, Path: "/", Domain: "chapmanganato.to"},
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error sending request: %v\n", err)
		return ""
	}
	defer resp.Body.Close()

	// fmt.Printf("Response Status: %s\n", resp.Status)
	// fmt.Printf("Content-Encoding: %s\n", resp.Header.Get("Content-Encoding"))

	var reader io.Reader

	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Printf("Error creating gzip reader: %v\n", err)
			return ""
		}
	case "br":
		reader = brotli.NewReader(resp.Body)
	default:
		reader = resp.Body
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		return ""
	}

	// // fmt.Printf("Decompressed body length: %d bytes\n", len(body))

	// // // Print the first 500 characters of the response body
	// // if len(body) > 500 {
	// // 	fmt.Printf("First 500 characters of decompressed response:\n%s\n", body[:500])
	// // } else {
	// // 	fmt.Printf("Full decompressed response:\n%s\n", body)
	// // }

	// Regular expression to match the image URLs in the specified format
	re := regexp.MustCompile(`<img[^>]+src=["']?(https://[a-zA-Z0-9]+\.([a-zA-Z0-9-]+\.com)/img/tab[^"'>]*)["']?`)

	// Find the first matching image URL
	matches := re.FindStringSubmatch(string(body))
	// fmt.Println(matches)
	if len(matches) == 0 {
		fmt.Println("No image server URL found in the response")
	}

	// Get the server URL from the matches
	match := matches[1] // This is the full URL matched

	serverURL, err := url.Parse(string(match))
	if err != nil {
		fmt.Printf("Error parsing server URL: %v\n", err)
		return ""
	}

	fmt.Printf("Found image server: %s\n", serverURL.Host)
	return serverURL.Host
}

// Check if the URL string looks like a base64-encoded string.
func isBase64Encoded(s string) bool {
	// Base64 strings are a multiple of 4 characters in length and contain only valid characters.
	if len(s)%4 != 0 {
		return false
	}
	base64Regex := regexp.MustCompile(`^[A-Za-z0-9+/=]+$`)
	return base64Regex.MatchString(s)
}

// Attempt to decode a base64 string. If successful, return the decoded string.
func tryBase64Decode(encoded string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("error decoding base64: %v", err)
	}
	return string(decoded), nil
}

func downloadFile(urlStr, filepath string) error {
	// Print the URL if -jp is enabled
	printJPUrl(urlStr)
	// Check if the URL string is Base64 encoded.
	if isBase64Encoded(urlStr) {
		decodedUrl, err := tryBase64Decode(urlStr)
		if err != nil {
			return fmt.Errorf("base64 decode error: %v", err)
		}
		urlStr = decodedUrl
	}
	// ///////////////TOR PROXY/////////////////////
	// // Set up the SOCKS5 proxy at localhost:347
	// socksProxy := "localhost:347"
	// dialer, err := proxy.SOCKS5("tcp", socksProxy, nil, proxy.Direct)
	// if err != nil {
	// 	return fmt.Errorf("error setting up SOCKS5 proxy: %v", err)
	// }

	// // Create a custom transport with the SOCKS5 dialer
	// transport := &http.Transport{
	// 	Dial: dialer.Dial,
	// }

	// // Create a client using the transport
	// client := &http.Client{
	// 	Transport: transport,
	// }

	// req, err := http.NewRequest("GET", urlStr, nil)
	// if err != nil {
	// 	return fmt.Errorf("error creating request: %v", err)
	// }
	/////////////////////////////////////////////
	// Disable if using proxy
	client := &http.Client{}
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}
	// Set headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:128.0) Gecko/20100101 Firefox/128.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/jxl,image/webp,image/png,image/svg+xml,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("DNT", "1")
	req.Header.Set("Sec-GPC", "1")
	// req.Header.Set("Referer", urlStr) // Wrong
	req.Header.Set("Referer", "https://chapmanganato.to/") // Important
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Priority", "u=0, i")
	req.Header.Set("TE", "trailers")
	req.Header.Set("Authority", "chapmanganato.to")

	// Set cookies
	cookies := []*http.Cookie{
		{Name: "ci_session", Value: "incw9QGKieHVVuooNkSm5vJS9ZUPuJmEVC7tNT2TgCUJYinu0EDzlEulbzmnRN98UC0KqSrN15gDmq6nHXSTe9kmAakV5fKtJ0xFRrNhRr5Xo%2FKVRHMrVGOADvScxP3G2KR1Z3XbMgU1CWw17DG01pElScdmS6riB9vbPfh8B2Euzj9rwfSodgnYDuE3Wqs46Pp0T8Odo%2FpLx5L%2FU4j0QJaiCmE6Yu%2FSoVy%2FBWg%2BWlDEJVLC3OrHlAnmG1b2DPlwTvgmiOHSgk27pl63zST7ppQusjKoy9tdNgHpQaoaejVyRNwXJv%2Bw8qMf%2B5j9hYlzs1m3nqSBwLx%2BIXoux8Q%2FF1Z6Y%2BzcbjdKJBMWeFeKPLHr%2F7DGACEsWTnsGw%2B%2BGycDQeJQjh8KmPx9nRIX9EiNKEyurpjADekCYmznE3JAVMq9NKkXSa72qJZ8gS0OYl12hUhoRMet1MFe1Q5RlM%2FB%2FA%3D%3Dedb59e6619acef07aacb5bbbaefbd98ae564d573", Path: "/", Domain: "chapmanganato.to"},
		{Name: "panel-fb-comment", Value: "fb-comment-title-show", Path: "/", Domain: "chapmanganato.to"},
		{Name: "content_lazyload", Value: "off", Path: "/", Domain: "chapmanganato.to"},
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Create a temporary file to store the downloaded image
	tempFile, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("file creation error: %v", err)
	}
	defer tempFile.Close()

	// Copy the response body to the temporary file
	if _, err = io.Copy(tempFile, resp.Body); err != nil {
		return fmt.Errorf("file write error: %v", err)
	}

	// Process the image if -jp is enabled
	if err := processJPImage(filepath); err != nil {
		return err
	}
	return nil
}

// Helper function to print the URL if -jp was passed
func printJPUrl(urlStr string) {
	if isJPMode {
		fmt.Printf("\n%s\r\n", urlStr)
	}
}

// Helper function to process the image if -jp was passed
func processJPImage(filepath string) error {
	if isJPMode {
		if err := processImage(filepath); err != nil {
			return fmt.Errorf("image processing error: %v", err)
		}
	}
	return nil
}

// func processImage(filepath string) error {

// 	// Identify the image format
// 	format, err := IdentifyImageFormat(filepath)
// 	if err != nil {
// 		return fmt.Errorf("error identifying image format: %v", err)
// 	}

// 	fmt.Printf("Detected image format: %s\n", format)

// 	// Open the original file
// 	origFile, err := os.Open(filepath)
// 	if err != nil {
// 		return fmt.Errorf("error opening image file: %v", err)
// 	}
// 	defer origFile.Close()

// 	// Declare the img variable
// 	var img image.Image

// 	// Get the original file size
// 	origInfo, err := origFile.Stat()
// 	if err != nil {
// 		return fmt.Errorf("error getting original file info: %v", err)
// 	}
// 	origSize := origInfo.Size()

// 	if format == "png" {
// 		// Decode PNG images first
// 		img, err = png.Decode(origFile)
// 		if err != nil {
// 			return fmt.Errorf("error decoding PNG image: %v", err)
// 		}
// 		// Convert PNG to JPEG and save it
// 		if err := convertToJpeg(img, filepath); err != nil {
// 			return err // Handle conversion errors
// 		}
// 	}
// 	// Now, open the newly created JPEG file
// 	origFile, err = os.Open(filepath) // Open the saved JPEG file for decoding
// 	if err != nil {
// 		return fmt.Errorf("error opening converted JPEG: %v", err)
// 	}
// 	if useFancyDecoding {
// 		// Use fancy decoding options
// 		decodingOptions := &jpegli.DecodingOptions{
// 			FancyUpsampling: true,
// 			BlockSmoothing:  true,
// 		}
// 		img, err = jpegli.DecodeWithOptions(origFile, decodingOptions)
// 		if err != nil {
// 			return fmt.Errorf("error decoding image with fancy options: %v", err)
// 		}
// 	} else {
// 		// Use standard decoding
// 		img, err = jpeg.Decode(origFile)
// 		if err != nil {
// 			return fmt.Errorf("error decoding image: %v", err)
// 		}
// 	}

// 	// Create a buffer to hold the new image data
// 	var buf bytes.Buffer

// 	checkJpegliQualityFlags()

// 	options := &jpegli.EncodingOptions{
// 		Quality: jpegliQuality, // Use the quality set from command-line arguments
// 	}
// 	if err := jpegli.Encode(&buf, img, options); err != nil {
// 		return fmt.Errorf("error encoding image with jpegli: %v", err)
// 	}

// 	// Get the new size after encoding
// 	newSize := int64(buf.Len())

// 	// Compare sizes and calculate the difference
// 	sizeDifference := newSize - origSize
// 	percentageChange := 0.0
// 	if origSize > 0 {
// 		percentageChange = (float64(sizeDifference) / float64(origSize)) * 100
// 	}

// 	// Determine if the file size increased, decreased, or stayed the same
// 	changeType := highlightStyle.Render("decreased")
// 	if sizeDifference > 0 {
// 		changeType = "increased"
// 	} else if sizeDifference == 0 {
// 		changeType = "remained the same"
// 	}

// 	// Output the file sizes and percentage change
// 	fmt.Printf("Processed image with jpegli: %s\n", filepath)
// 	fmt.Printf("Original file size: %d bytes\n", origSize)
// 	fmt.Printf("New file size: %d bytes\n", newSize)
// 	fmt.Printf("Size difference: %d bytes (%s)\n", sizeDifference, changeType)
// 	fmt.Printf("Percentage change: %.2f%%\n", percentageChange)

// 	// If the new size is smaller, write it back to the original file
// 	if newSize < origSize {
// 		if err := os.WriteFile(filepath, buf.Bytes(), 0644); err != nil {
// 			return fmt.Errorf("error writing processed image: %v", err)
// 		}
// 		fmt.Println("New image file saved as it is smaller than the original.")
// 	} else {
// 		fmt.Println("New file size is not smaller. Keeping original file.")
// 	}

// 	return nil
// }

func processImage(filepath string) error {
	format, err := identifyFormat(filepath)
	if err != nil {
		return fmt.Errorf("error identifying image format: %v", err)
	}
	fmt.Printf("Detected image format: %s\n", format)

	origFile, err := openFile(filepath)
	if err != nil {
		return err
	}
	defer origFile.Close()

	origSize, img, err := handleOriginalFile(origFile, format)
	if err != nil {
		return err
	}

	if err := convertImageIfNeeded(format, filepath, &img); err != nil {
		return err
	}

	origFile, err = os.Open(filepath)
	if err != nil {
		return fmt.Errorf("error opening converted JPEG: %v", err)
	}

	img, err = decodeImage(origFile, useFancyDecoding)
	if err != nil {
		return err
	}

	return encodeAndCompareSizes(filepath, origSize, img)

}

func identifyFormat(filepath string) (string, error) {
	return IdentifyImageFormat(filepath)
}

func openFile(filepath string) (*os.File, error) {
	return os.Open(filepath)
}

func handleOriginalFile(origFile *os.File, format string) (int64, image.Image, error) {
	origInfo, err := origFile.Stat()
	if err != nil {
		return 0, nil, fmt.Errorf("error getting original file info: %v", err)
	}
	origSize := origInfo.Size()

	var img image.Image
	if format == "png" {
		img, err = png.Decode(origFile)
		if err != nil {
			return 0, nil, fmt.Errorf("error decoding PNG image: %v", err)
		}
	}

	return origSize, img, nil
}

func convertImageIfNeeded(format, filepath string, img *image.Image) error {
	if format == "png" {
		return convertToJpeg(*img, filepath)
	}
	return nil
}

func decodeImage(origFile *os.File, useFancy bool) (image.Image, error) {
	var img image.Image
	var err error
	if useFancy {
		options := &jpegli.DecodingOptions{FancyUpsampling: true, BlockSmoothing: true}
		img, err = jpegli.DecodeWithOptions(origFile, options)
	} else {
		img, err = jpeg.Decode(origFile)
	}
	if err != nil {
		return nil, fmt.Errorf("error decoding image: %v", err)
	}
	return img, nil
}

func encodeAndCompareSizes(filepath string, origSize int64, img image.Image) error {
	var buf bytes.Buffer
	options := &jpegli.EncodingOptions{Quality: jpegliQuality}
	if err := jpegli.Encode(&buf, img, options); err != nil {
		return fmt.Errorf("error encoding image with jpegli: %v", err)
	}

	newSize := int64(buf.Len())
	sizeDifference := newSize - origSize
	percentageChange := 0.0
	if origSize > 0 {
		percentageChange = (float64(sizeDifference) / float64(origSize)) * 100
	}

	changeType := highlightStyle.Render("decreased")
	if sizeDifference > 0 {
		changeType = "increased"
	} else if sizeDifference == 0 {
		changeType = "remained the same"
	}

	fmt.Printf("Processed image with jpegli: %s\n", filepath)
	fmt.Printf("Original file size: %d bytes\n", origSize)
	fmt.Printf("New file size: %d bytes\n", newSize)
	fmt.Printf("Size difference: %d bytes (%s)\n", sizeDifference, changeType)
	fmt.Printf("Percentage change: %.2f%%\n", percentageChange)

	if newSize < origSize {
		if err := os.WriteFile(filepath, buf.Bytes(), 0644); err != nil {
			return fmt.Errorf("error writing processed image: %v", err)
		}
		fmt.Println("New image file saved as it is smaller than the original.")
	} else {
		fmt.Println("New file size is not smaller. Keeping original file.")
	}

	return nil
}

// IdentifyImageFormat checks the magic numbers to determine the image format.
func IdentifyImageFormat(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	// Read the first few bytes of the file
	header := make([]byte, 8)
	_, err = file.Read(header)
	if err != nil {
		return "", fmt.Errorf("error reading file header: %v", err)
	}

	// Check the magic number for image formats
	if strings.HasPrefix(string(header), "\x89PNG") {
		return "png", nil
	} else if header[0] == 0xFF && header[1] == 0xD8 {
		return "jpeg", nil
	} else if header[0] == 0x49 && header[1] == 0x49 && header[2] == 0x2A && header[3] == 0x00 {
		return "tiff", nil
	} else if header[0] == 0x4D && header[1] == 0x5A {
		return "bmp", nil
	}

	return "unknown", nil
}

// convertToJpeg converts an image.Image to JPEG and saves it to the original file path
func convertToJpeg(img image.Image, filepath string) error {
	// Create a buffer to hold the JPEG data
	var buf bytes.Buffer

	// Encode the image as JPEG
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}) // Adjust quality as needed
	if err != nil {
		return fmt.Errorf("error encoding image to JPEG: %v", err)
	}

	// Write the JPEG data back to the file
	err = os.WriteFile(filepath, buf.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("error writing JPEG file: %v", err)
	}

	fmt.Printf("Converted image to JPEG and saved: %s\n", filepath)
	return nil
}

func checkJpegliQualityFlags() {
	for i, arg := range os.Args[1:] {
		if arg == "-jp" || arg == "--jpegli" {
			isJPMode = true
		}
		if (arg == "-q" || arg == "--quality") && i+1 < len(os.Args[1:]) {
			// Attempt to parse the next argument as the quality value
			qualityArg := os.Args[i+2] // The quality value comes after the flag
			parsedQuality, err := strconv.Atoi(qualityArg)
			if err == nil {
				jpegliQuality = parsedQuality // Set the quality if parsing succeeds
				fmt.Println("Using quality: ", jpegliQuality)
			} else {
				fmt.Println("Invalid quality value. Using default quality.")
				jpegliQuality = 85
			}
		}
	}
}

func getFileSize(filePath string) (int64, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	return fileInfo.Size(), nil
}

func calculatePercentageChange(beforeSize, afterSize int64) float64 {
	return float64(afterSize-beforeSize) / float64(beforeSize) * 100
}

func getDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func verifyImage(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		log.Printf("Error opening image file %s: %v", path, err)
		return false
	}
	defer file.Close()

	_, format, err := image.DecodeConfig(file)
	if err != nil {
		log.Printf("Error decoding image %s: %v", path, err)
		return false
	}

	// Check if the format is one of the allowed types
	return format == "jpeg" || format == "png" || format == "gif"
}

func recordBrowseHistory(filename string, record BrowseRecord) error {
	var records []BrowseRecord

	// Set the maximum allowed file size (e.g., 5 MB)
	const maxFileSize = 5 * 1024 * 1024 // 5 MB

	// Get the file size
	fileInfo, err := os.Stat(filename)
	if err == nil && fileInfo.Size() >= maxFileSize {
		// Archive old file by renaming it with a timestamp
		archiveFilename := fmt.Sprintf("%s_%s.json", filename, time.Now().Format("20060102_150405"))
		if err := os.Rename(filename, archiveFilename); err != nil {
			return fmt.Errorf("error archiving file: %v", err)
		}
	}

	// Try to read the existing history file
	fileData, err := os.ReadFile(filename)
	if err == nil {
		// If the file exists, unmarshal the existing records into the slice
		if err := json.Unmarshal(fileData, &records); err != nil {
			return fmt.Errorf("error unmarshaling existing data: %v", err)
		}
	}

	// Set the current time in the new record
	record.Timestamp = time.Now()

	// Append the new record to the records slice
	records = append(records, record)

	// Marshal the updated records slice back into JSON
	updatedData, err := json.MarshalIndent(records, "", "    ") // Indented for readability
	if err != nil {
		return fmt.Errorf("error marshaling updated records: %v", err)
	}

	// Write the updated data back to the file (overwrite the file)
	if err := os.WriteFile(filename, updatedData, 0644); err != nil {
		return fmt.Errorf("error writing updated data to file: %v", err)
	}

	return nil
}

func browseHistory(filename string) ([]BrowseRecord, error) {
	var records []BrowseRecord

	fileData, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	if err := json.Unmarshal(fileData, &records); err != nil {
		return nil, fmt.Errorf("error unmarshaling record: %v", err)
	}

	return records, nil
}

func showHistory() {
	records, err := browseHistory(historyFile)
	if err != nil {
		fmt.Println("Error fetching browse history:", err)
		return
	}

	if len(records) > 0 {
		latestRecord := records[len(records)-1]
		fmt.Printf("Most recent record:\n %s\n Chapter: %d\n Chapter Title: %s\n Url: %s\n Date: %s\n",
			latestRecord.MangaTitle,
			latestRecord.ChapterNumber,
			latestRecord.ChapterTitle,
			latestRecord.ChapterPage,
			latestRecord.Timestamp.Format(time.RFC1123)) // Format the timestamp
	} else {
		fmt.Println("No browse history found.")
	}
}

func openLastSession(filename string) error {
	var records []BrowseRecord

	fileData, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}

	if err := json.Unmarshal(fileData, &records); err != nil {
		return fmt.Errorf("error unmarshaling data: %v", err)
	}

	if len(records) == 0 {
		return fmt.Errorf("no browse history found")
	}

	// Get the last record (most recent session)
	lastRecord := records[len(records)-1]

	fmt.Printf("Resuming session for Manga: %s, Chapter: %d, Page: %s\n",
		lastRecord.MangaTitle, lastRecord.ChapterNumber, lastRecord.ChapterPage)

	manga := MangaResult{Title: lastRecord.MangaTitle}
	chapter := Chapter{Number: lastRecord.ChapterNumber, URL: lastRecord.ChapterPage}

	//////////////////////////////////////////////////////////
	// Check if pdf exists
	/////////////////////////////////////////////////
	mangaDir := filepath.Join(cacheDir, getModMangaTitle(manga.Title))
	pdfFilename := fmt.Sprintf("%s.pdf", lastRecord.ChapterTitle)
	pdfPath := filepath.Join(mangaDir, pdfFilename)
	// Return if PDF already exists
	if _, err := os.Stat(pdfPath); err == nil {
		openPDF(pdfPath)
	} else {
		images, chapterTitle := scrapeChapterImages(lastRecord.ChapterPage)
		pdfPath = downloadAndConvertToPDF(manga, chapter, images, chapterTitle)
	}
	/////////////////////////////////////////////////////////

	openPDF(pdfPath)
	lastRecord.ChapterPage = strings.Join(strings.Split(lastRecord.ChapterPage, "/")[:len(strings.Split(lastRecord.ChapterPage, "/"))-1], "/")

	chapters := scrapeChapterList(lastRecord.ChapterPage)
	if len(chapters) == 0 {
		fmt.Println("No chapters found. Exiting...")
		os.Exit(1)
	}

	inputControls(manga, chapters, chapter)
	return nil
}

func initialModel() model {
	records, _ := browseHistory(historyFile)
	return model{
		records: records,
		// cursor:  0,
		cursor: len(records) - 1, // Set cursor to the last record
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	case tea.MouseEvent:
		return m.handleMouseEvent(msg)
	}
	return m, nil
}

func (m model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down":
		if m.cursor < len(m.records)-1 {
			m.cursor++
		}
	case "pgup":
		m.cursor -= 10
		if m.cursor < 0 {
			m.cursor = 0
		}
	case "pgdown":
		m.cursor += 10
		if m.cursor >= len(m.records) {
			m.cursor = len(m.records) - 1
		}
	case "home":
		m.cursor = 0
	case "end":
		m.cursor = len(m.records) - 1
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) handleMouseEvent(msg tea.MouseEvent) (tea.Model, tea.Cmd) {
	if msg.IsWheel() {
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if m.cursor > 0 {
				m.cursor--
			}
		case tea.MouseButtonWheelDown:
			if m.cursor < len(m.records)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	// Define styles for different sections
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Bold(true) // Selected row background magenta
	// defaultStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))                                                // Default white text for non-selected rows

	// Specific colors for selected sections
	mangaTitleStyleSelected := lipgloss.NewStyle().Foreground(lipgloss.Color("228")).Background(lipgloss.Color("57")).Bold(true)    // Yellow for Manga Title
	chapterNumberStyleSelected := lipgloss.NewStyle().Foreground(lipgloss.Color("112")).Background(lipgloss.Color("57")).Bold(true) // Green for Chapter Number
	chapterTitleStyleSelected := lipgloss.NewStyle().Foreground(lipgloss.Color("177")).Background(lipgloss.Color("57")).Bold(true)  // Light purple for Chapter Title
	chapterUrlStyleSelected := lipgloss.NewStyle().Foreground(lipgloss.Color("87"))                                                 // Bright cyan for selected state
	timestampStyleSelected := lipgloss.NewStyle().Foreground(lipgloss.Color("103")).Background(lipgloss.Color("57")).Bold(true)     // Cyan for Timestamp
	// Specific colors for sections
	mangaTitleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("228"))    // Yellow for Manga Title
	chapterNumberStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("112")) // Green for Chapter Number
	chapterTitleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("177"))  // Light purple for Chapter Title
	chapterUrlStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("177"))    // Soft pink for default state

	timestampStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("103")) // Cyan for Timestamp

	s := "Browse History:\n\n"
	for i, record := range m.records {
		var row string

		// Check if it's the currently selected row (with the cursor)
		if m.cursor == i {
			row = selectedStyle.Render(fmt.Sprintf(" > %s - Chapter %s: %s %s (%s)",
				mangaTitleStyleSelected.Render(record.MangaTitle),
				chapterNumberStyleSelected.Render(fmt.Sprintf("%d", record.ChapterNumber)),
				chapterTitleStyleSelected.Render(record.ChapterTitle),
				chapterUrlStyleSelected.Render(record.ChapterPage),
				timestampStyleSelected.Render(record.Timestamp.Format(time.RFC1123)),
			))
		} else {
			row = fmt.Sprintf("   %s - Chapter %s: %s %s (%s)",
				mangaTitleStyle.Render(record.MangaTitle),
				chapterNumberStyle.Render(fmt.Sprintf("%d", record.ChapterNumber)),
				chapterTitleStyle.Render(record.ChapterTitle),
				chapterUrlStyle.Render(record.ChapterPage),
				timestampStyle.Render(record.Timestamp.Format(time.RFC1123)),
			)
		}

		s += row + "\n"
	}

	s += "\nPress 'q' to quit.\n"
	return s
}

func lookHistory() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}

func showHistoryWithTablewriter() {
	records, err := browseHistory(historyFile)
	if err != nil {
		fmt.Println("Error fetching browse history:", err)
		return
	}

	// Create a table writer
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Manga Title", "Chapter", "Chapter Title", "Url", "Date"})

	// 	Explanation of Color Choices:
	// Header: Bold and Bright Cyan (FgHiCyan) to make the header stand out.
	// Columns:
	// Manga Title: Yellow (FgYellow) for visibility and emphasis.
	// Chapter: Green (FgGreen) for a calming contrast.
	// Chapter Title: Magenta (FgMagenta) for vibrancy.
	// URL: Blue (FgBlue) to signify links.
	// Date: Bright white (FgHiWhite) to keep it neutral but clear.
	// Set header color: Bright cyan

	table.SetHeaderColor(
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiCyanColor},
	)

	table.SetColumnColor(
		tablewriter.Colors{tablewriter.FgYellowColor},  // Manga Title: Yellow
		tablewriter.Colors{tablewriter.FgGreenColor},   // Chapter: Green
		tablewriter.Colors{tablewriter.FgMagentaColor}, // Chapter Title: Magenta
		tablewriter.Colors{tablewriter.FgBlueColor},    // URL: Blue
		tablewriter.Colors{tablewriter.FgHiWhiteColor}, // Date: White or Gray
	)

	for _, record := range records {
		table.Append([]string{
			record.MangaTitle,
			fmt.Sprintf("%d", record.ChapterNumber),
			record.ChapterTitle,
			record.ChapterPage,
			record.Timestamp.Format(time.RFC1123),
		})
	}

	// Render the table to output
	table.Render()
}

// Checks if "-jp" was passed in the command-line arguments
func checkJPFlag() {
	for _, arg := range os.Args[1:] {
		if arg == "-jp" || arg == "--jpegli" {
			isJPMode = true
			break
		}
	}
}

// Checks if "-C" "--clear-cache" was passed in the command-line arguments
func checkCCacheFlag() {
	// Check if there are any flags related to cache mode
	for _, arg := range os.Args[1:] {
		if arg == "-C" || arg == "--clear-cache" {
			isCCacheMode = false
			break // Exit the loop once we find the flag
		}
	}

	// Check if there are additional parameters
	if len(os.Args) > 2 { // More than just the program name and one flag
		isCCacheMode = true
	}
}
