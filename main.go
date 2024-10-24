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
	"github.com/charmbracelet/lipgloss"
	"github.com/gen2brain/jpegli"
	"github.com/go-pdf/fpdf"

	fzf "github.com/koki-develop/go-fzf"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/image/webp"
	"golang.org/x/sync/semaphore"
	"golang.org/x/time/rate"
)

const (
	version     = "0.1.46"
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

type MangaStatistics struct {
	TotalChapters      int
	ReadChapters       int
	LastReadChapter    BrowseRecord
	OldestTimestamp    time.Time
	NewestTimestamp    time.Time
	ReadingDates       []time.Time
	ChaptersNotRead    int
	UniqueChaptersRead map[int]bool // Track unique chapter numbers read
	MostReadCount      int
}

type model struct {
	records []BrowseRecord
	cursor  int
}

var (
	cacheDir           string // Directory to hold files, preferably temp
	currentManga       string
	servers            = []string{"server2", "server1"} // Switch between content servers serving media
	contentServer      string
	isJPMode           bool        // check whether user wants jpegli enabled
	isWideSplitMode    bool        // check whether user wants to split wide images or scale to A4
	isCCacheMode       bool        // This check is done so we don't print storage size when inside program since it is called in inputControls()
	useFancyDecoding        = true // Flag for toggling decoding method
	jpegliQuality      int  = 85   // Default quality for jpegli encoding
	lightMagentaStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF79C6"))
	lightMagentaWithBg      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF79C6")).Background(lipgloss.Color("#00194f"))
	lightCyanStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD"))
	textStyle               = lipgloss.NewStyle().Foreground(lipgloss.Color("#F8F8F2"))
	redStyle                = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	magentaStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF00FF"))
	yellowStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("#ebeb00"))
	greenStyle              = lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B"))
	cyanColor               = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FFFF"))
	inputStyle              = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFB86C"))
	versionStyle            = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFB86C")).Background(lipgloss.Color("#282A36")).Padding(0, 2) // Adds horizontal padding to the version text
	headerStyle             = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8BE9FD")).Background(lipgloss.Color("#282A36")).Padding(0, 2)
	resultStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B")).Padding(0, 2)
	indexStyle              = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6")).PaddingRight(1)
	bracketStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6"))
	chapterStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("#95fb17"))
	chapterStyleWithBG      = lipgloss.NewStyle().Foreground(lipgloss.Color("#95fb17")).Background(lipgloss.Color("#282A36"))
	yellowFGbrownBG         = lipgloss.NewStyle().Foreground(lipgloss.Color("#95fb17")).Background(lipgloss.Color("#282A36"))
	redFGblackBG            = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5f56")).Background(lipgloss.Color("#1e1e1e"))
	// blueFGpurpleBG          = lipgloss.NewStyle().Foreground(lipgloss.Color("#005f87")).Background(lipgloss.Color("#4B0082"))
	blueFGpurpleBG   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffff00")).Background(lipgloss.Color("#00194f"))
	greenFGwhiteBG   = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")).Background(lipgloss.Color("#ffffff"))
	cyanFGdarkBlueBG = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ffff")).Background(lipgloss.Color("#000080"))
	resetStyle       = lipgloss.NewStyle()
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	debug.SetMaxStack(1000000000)

	checkJPFlag()
	checkWideSplitFlag()
	checkCCacheFlag()
	checkCacheDir()
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
	case "-ws", "--wide-split":
		isWideSplitMode = true
	case "-H", "--history":
		showHistory()
	case "-bh", "--browse-history":
		showHistoryWithFzf()
	case "-r", "--resume":
		openLastSession(historyFile)
	case "-od", "--opendir":
		checkCacheDir()
		err := openDirectory(cacheDir)
		if err != nil {
			fmt.Println("Error opening directory:", err)
		}
	case "-st", "--stats":
		fetchStatistics()
	case "-c", "--cache-size":
		showCacheSize()
	case "-C", "--clear-cache":
		clearCache()
	case "-f", "--fix":
		removeEmptyEntries(historyFile)
	default:
		searchAndReadManga()
	}
}

func fetchStatistics() {
	var records []BrowseRecord
	fileData, err := os.ReadFile(historyFile)
	if err != nil {
		return
	}

	if err := json.Unmarshal(fileData, &records); err != nil {
		fmt.Printf("error unmarshaling data from historyFile: %v\n", err)
	}

	// Process records from additional JSON files
	globPattern := "goreadmanga_history_*.json"
	matches, err := filepath.Glob(globPattern)
	if err != nil {
		fmt.Printf("error finding additional history files: %v\n", err)
	}

	for _, filePath := range matches {
		fileData, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("Error reading file %s: %v\n", filePath, err)
			continue // skip to next file if reading fails
		}
		var additionalEntries []BrowseRecord
		if err := json.Unmarshal(fileData, &additionalEntries); err != nil {
			fmt.Printf("Error unmarshaling data from %s: %v\n", filePath, err)
			continue // skip to next file if unmarshalling fails
		}
		records = append(records, additionalEntries...)
	}

	calculateStatistics(records)
}

func showHelp() {
	title := lightMagentaStyle.Render("goreadmanga " + version + " (github.com/stl3/GoReadManga)")
	subtitle := lightCyanStyle.Render("App for finding manga via the terminal")
	usage := greenStyle.Render("Usage:")
	options := greenStyle.Render("Options:")
	optionText := textStyle.Render(`
  -h, --help             Print this help page
  -v, --version          Print version number
  -jp, --jpegli          Use jpegli to re-encode jpegs
  -q, --quality		  Set quality to use with jpegli encoding (default: 85)
  -ws, --wide-split      Split images that are too wide and maximize vertically
  -H, --history   	   Show last viewed manga entry in history
  -bh, --browse-history  Browse history file, select and read
  -st, --stats           Show history statistics
  -r, --resume   	    Continue from last session
  -od, --opendir         Open pdf dir
  -c, --cache-size       Print cache size (` + cacheDir + `)
  -C, --clear-cache      Purge cache dir (` + cacheDir + `)
  -f, --fix		      Remove json entries causing problems (empty chapter_page/chapter_title during network issues)
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
		fmt.Printf("Error or cache already empty: %v\n", err)
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
			fmt.Printf("Error or cache already empty: %v\n", err)
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
	// Not sure if what you're actually searching is beyond 1st 5 pages...
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
	// totalChapters := len(chapters) // Add a record for total chapters so we can generate statistics
	// Create a record for the current manga and chapter
	record := BrowseRecord{
		MangaTitle:    manga.Title,
		ChapterNumber: chapter.Number,
		ChapterTitle:  chapterTitle,
		ChapterPage:   chapter.URL,
		// TotalChapters: totalChapters,
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
	// chapterTitle contains title/chapter number/chapter title
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
	pdf := fpdf.New("P", "mm", "A4", "")
	pageWidth, pageHeight := pdf.GetPageSize()

	for _, imagePath := range imagePaths {
		file, err := os.Open(imagePath)
		if err != nil {
			return fmt.Errorf("error opening image %s: %v", imagePath, err)
		}
		imgConfig, _, err := image.DecodeConfig(file)
		file.Close()
		if err != nil {
			return fmt.Errorf("error decoding image %s: %v", imagePath, err)
		}

		// Calculate aspect ratios
		imageRatio := float64(imgConfig.Height) / float64(imgConfig.Width)
		pageRatio := pageHeight / pageWidth

		// Check if image is very tall
		if imageRatio > (2 * pageRatio) {
			// Handle tall image (existing code)
			scale := pageWidth / float64(imgConfig.Width)
			scaledWidth := float64(imgConfig.Width) * scale
			scaledHeight := float64(imgConfig.Height) * scale
			numPages := int(math.Ceil(scaledHeight / pageHeight))

			for page := 0; page < numPages; page++ {
				pdf.AddPage()
				pdf.SetFillColor(0, 0, 0)
				pdf.Rect(0, 0, pageWidth, pageHeight, "F")
				yOffset := float64(page) * pageHeight
				x := (pageWidth - scaledWidth) / 2
				pdf.Image(imagePath, x, -yOffset, scaledWidth, scaledHeight, false, "", 0, "")
			}
		} else if isWideSplitMode { // Perform only if wide split mode specified
			if float64(imgConfig.Width)/pageWidth > 1.5 {
				// Handling horizontally wider image by splitting it horizontally
				scale := pageHeight / float64(imgConfig.Height)
				scaledWidth := float64(imgConfig.Width) * scale
				scaledHeight := float64(imgConfig.Height) * scale

				// Determine number of splits needed based on scaled width
				numSplits := int(math.Ceil(scaledWidth / pageWidth))

				// Calculate the exact width each slice should cover
				sliceWidth := scaledWidth / float64(numSplits)

				// Image options
				var opt fpdf.ImageOptions
				opt.AllowNegativePosition = true

				// Calculate vertical centering once
				yPosition := (pageHeight - scaledHeight) / 2

				// Handle each split
				for split := 0; split < numSplits; split++ {
					pdf.AddPage()

					// Set background [black]
					pdf.SetFillColor(0, 0, 0)
					pdf.Rect(0, 0, pageWidth, pageHeight, "F")

					// Calculate horizontal position for current split
					// Use sliceWidth instead of pageWidth for more precise splitting
					xOffset := float64(split) * sliceWidth

					// Add image with proper positioning
					pdf.ImageOptions(
						imagePath,
						-xOffset,
						yPosition,
						scaledWidth,
						scaledHeight,
						false,
						opt,
						0,
						"")
				}

			}
		} else {
			// Handle normal images
			pdf.AddPage()
			pdf.SetFillColor(0, 0, 0)
			pdf.Rect(0, 0, pageWidth, pageHeight, "F")

			scaleX := pageWidth / float64(imgConfig.Width)
			scaleY := pageHeight / float64(imgConfig.Height)
			scale := math.Min(scaleX, scaleY)

			width := float64(imgConfig.Width) * scale
			height := float64(imgConfig.Height) * scale

			x := (pageWidth - width) / 2
			y := (pageHeight - height) / 2

			pdf.Image(imagePath, x, y, width, height, false, "", 0, "")
		}
	}

	return pdf.OutputFileAndClose(outputPath)
}

func openPDF(pdfPath string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "android":
		// Use an Intent to open the PDF file in Termux
		// Needs fixing, doesn't open file browser
		cmd = exec.Command("termux-open", pdfPath)

	case "darwin": // macOS
		cmd = exec.Command("open", pdfPath)

	case "windows":
		// Check if SumatraPDF is available, just for me
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
				greenStyle.Render("Chapter") +
				chapterStyleWithBG.Render(fmt.Sprintf(" %d/%d", currentChapterNumber, totalChapters)) +
				bracketStyle.Render("] ") +
				lightMagentaWithBg.Render("▄︻デ══━一 🌟💥 ", chapterTitle, " 💥🌟"),
		)
		fmt.Println(bracketStyle.Render("[") + greenStyle.Render("N") + bracketStyle.Render("]") + textStyle.Render(" Next chapter"))
		fmt.Println(bracketStyle.Render("[") + greenStyle.Render("P") + bracketStyle.Render("]") + textStyle.Render(" Previous chapter"))
		fmt.Println(bracketStyle.Render("[") + greenStyle.Render("S") + bracketStyle.Render("]") + textStyle.Render(" Select chapter"))
		fmt.Println(bracketStyle.Render("[") + greenStyle.Render("R") + bracketStyle.Render("]") + textStyle.Render(" Reopen current chapter"))
		fmt.Println(bracketStyle.Render("[") + greenStyle.Render("A") + bracketStyle.Render("]") + textStyle.Render(" Search another manga"))
		fmt.Println(bracketStyle.Render("[") + greenStyle.Render("BH") + bracketStyle.Render("]") + textStyle.Render(" Browse history, select to read"))
		fmt.Println(bracketStyle.Render("[") + greenStyle.Render("ST") + bracketStyle.Render("]") + textStyle.Render(" See stats"))
		fmt.Println(bracketStyle.Render("[") + greenStyle.Render("OD") + bracketStyle.Render("]") + textStyle.Render(" Open PDF dir"))
		fmt.Println(bracketStyle.Render("[") + greenStyle.Render("CS") + bracketStyle.Render("]") + textStyle.Render(" Toggle between content server1/2"))
		fmt.Println(bracketStyle.Render("[") + greenStyle.Render("D") + bracketStyle.Render("]") + textStyle.Render(" Toggle image decoding method [jpegli/normal]"))
		fmt.Println(bracketStyle.Render("[") + greenStyle.Render("M") + bracketStyle.Render("]") + textStyle.Render(" Toggle jpegli encoding mode [jpegli/normal]"))
		fmt.Println(bracketStyle.Render("[") + greenStyle.Render("WS") + bracketStyle.Render("]") + textStyle.Render(" Toggle splitting images wider than page"))
		fmt.Println(bracketStyle.Render("[") + greenStyle.Render("C") + bracketStyle.Render("]") + textStyle.Render(" Clear cache"))
		fmt.Println(bracketStyle.Render("[") + greenStyle.Render("Q") + bracketStyle.Render("]") + textStyle.Render(" Exit"))
		showCacheSize()
		var jpConfig, decodeConfig, jpegliQualityConfig, widesplitConfig string
		if isJPMode {
			jpConfig = "Jpegli"
		} else {
			jpConfig = "Standard"
		}
		if useFancyDecoding {
			decodeConfig = "Jpegli"
		} else {
			decodeConfig = "Standard"
		}
		if isWideSplitMode {
			widesplitConfig = "ON"
		} else {
			widesplitConfig = "OFF"
		}
		serverConfig := servers[0]
		jpegliQualityConfig = fmt.Sprintf("%d", jpegliQuality)
		fmt.Println(cyanColor.Render("Current options: ") +
			greenStyle.Render("Server") + bracketStyle.Render("[") + chapterStyleWithBG.Render(serverConfig) + bracketStyle.Render("] ") +
			greenStyle.Render("Decoding") + bracketStyle.Render("[") + chapterStyleWithBG.Render(decodeConfig) + bracketStyle.Render("] ") +
			greenStyle.Render("Encode") + bracketStyle.Render("[") + chapterStyleWithBG.Render(jpConfig) + bracketStyle.Render("] ") +
			greenStyle.Render("Quality") + bracketStyle.Render("[") + chapterStyleWithBG.Render(jpegliQualityConfig) + bracketStyle.Render("] ") +
			greenStyle.Render("Wide-split") + bracketStyle.Render("[") + chapterStyleWithBG.Render(widesplitConfig) + bracketStyle.Render("] "))
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
		case "bh":
			showHistoryWithFzf()
		case "st":
			fetchStatistics()
		case "od":
			checkCacheDir()
			err := openDirectory(cacheDir)
			if err != nil {
				fmt.Println("Error opening directory:", err)
			}
		case "cs":
			changeServerOrder()
		case "d":
			toggleDecodingMethod()
		case "m":
			isJPMode = !isJPMode
			displayEncodingStatus()
		case "ws":
			isWideSplitMode = !isWideSplitMode
			displayWideSplitStatus()
			// Better to just clear cache manually from here
			// rather than implementing bizarro code in clearCache()
			err := os.RemoveAll(cacheDir)
			if err != nil {
				fmt.Printf("Error clearing cache: %v\n", err)
			} else {
				fmt.Println("💥💥 Cache cleared due to mode change 💥💥")
			}
		case "c":
			clearCache()
		case "q":
			os.Exit(0)
		default:
			fmt.Println(lightCyanStyle.Render("Invalid input, please try again."))
		}
	}

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

// Function to display wide-split mode
func displayWideSplitStatus() {
	if isWideSplitMode {
		fmt.Println("✔️✔️ ⚡⚡ " + indexStyle.Render("Wide-split mode active") + " ⚡⚡ ✔️✔️")
	} else {
		fmt.Println("❌❌ " + indexStyle.Render("Wide-split mode deactivated") + " ❌❌")
	}
}

func checkIfPDFExist(manga MangaResult, chapterTitle string, cacheDir string, currentChapter Chapter) {
	mangaDir := filepath.Join(cacheDir, getModMangaTitle(manga.Title))
	chapterTitle = sanitizeFilename(chapterTitle)
	pdfFilename := fmt.Sprintf("%s.pdf", chapterTitle)
	pdfPath := filepath.Join(mangaDir, pdfFilename)

	// Return if PDF already exists
	if _, err := os.Stat(pdfPath); err == nil {
		fmt.Printf(yellowStyle.Render("PDF already exists: %s\n"), pdfPath)
		openPDF(pdfPath)
	} else {
		// fmt.Printf(infoStyle.Render("PDF doesn't exist: %s\n", pdfPath))
		openChapter(manga, currentChapter)
	}
}

// Function to toggle the decoding method
func toggleDecodingMethod() {
	useFancyDecoding = !useFancyDecoding // Toggle the flag
	if useFancyDecoding {
		fmt.Println(yellowStyle.Render("Using fancy decoding options."))
	} else {
		fmt.Println(yellowStyle.Render("Using standard decoding."))
	}
}

// Function to change the server order based on a switch case
func changeServerOrder() {
	if servers[0] == "server1" {
		// Toggle the order
		servers = []string{"server2", "server1"}
		fmt.Println(yellowStyle.Render("Switched server2"))
	} else {
		// Toggle back to the original order
		servers = []string{"server1", "server2"}
		fmt.Println(yellowStyle.Render("Switched server1"))
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
	if err := processImage(filepath); err != nil {
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

func processImage(filepath string) error {
	format, err := IdentifyImageFormat(filepath)
	if err != nil {
		return fmt.Errorf("error identifying image format: %v", err)
	}
	if isJPMode {
		fmt.Printf("Detected image format: %s\n", yellowStyle.Render(format))
	}

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
	if isJPMode {
		return encodeAndCompareSizes(filepath, origSize, img)
	} else {
		return err
	}
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

	switch format {
	case "png":
		img, err = png.Decode(origFile)
		if err != nil {
			return 0, nil, fmt.Errorf("error decoding PNG image: %v", err)
		}
	case "webp":
		img, err = webp.Decode(origFile)
		// img, err = webp.Decode(origFile, &decoder.Options{}) // for github.com/kolesa-team/go-webp
		if err != nil {
			return 0, nil, fmt.Errorf("error decoding WebP image: %v", err)
		}
	}

	return origSize, img, nil
}

func convertImageIfNeeded(format, filepath string, img *image.Image) error {
	if format == "png" || format == "webp" {
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

	changeType := greenStyle.Render("decreased")
	if sizeDifference > 0 {
		changeType = redStyle.Render("increased")
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

func IdentifyImageFormat(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	// Read the first few bytes of the file
	header := make([]byte, 12) // Increase size to 12 bytes
	_, err = file.Read(header)
	if err != nil {
		return "", fmt.Errorf("error reading file header: %v", err)
	}

	// Check the magic number for image formats
	if strings.HasPrefix(string(header), "\x89PNG") {
		return "png", nil
	} else if header[0] == 0xFF && header[1] == 0xD8 {
		return "jpeg", nil
	} else if header[0] == 0x49 && header[1] == 0x49 && header[2] == 0x2A {
		return "tiff", nil
	} else if header[0] == 0x4D && header[1] == 0x5A {
		return "bmp", nil
	} else if string(header[:4]) == "RIFF" && string(header[8:12]) == "WEBP" {
		return "webp", nil
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
	if isJPMode {
		fmt.Printf("Converted image to JPEG and saved: %s\n", filepath)
	}
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
	return format == "jpeg" || format == "png" || format == "webp" || format == "gif"
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

func showHistoryWithFzf() {
	records, err := browseHistory(historyFile) // Replace with actual path
	if err != nil {
		log.Fatal("Error fetching browse history:", err)
	}

	// Reverse the order of records (newer items at the bottom)
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	var selectedRecords []int

	// Check if native fzf is available
	if isFzfAvailable() {
		fmt.Println("Using native fzf...")
		selectedRecords, err = selectHistoryWithFzfNative(records)
	} else {
		fmt.Println("Using built-in go-fzf...")
		selectedRecords, err = selectHistoryWithGoFzf(records)
	}

	if err != nil {
		log.Fatal("Error selecting history:", err)
	}

	// Handle the selected entries (printing the selection)
	for _, i := range selectedRecords {
		selectedRecord := records[i]
		fmt.Printf(">: %s, Chapter %d: %s %s\n",
			magentaStyle.Render(selectedRecord.MangaTitle),
			selectedRecord.ChapterNumber,
			selectedRecord.ChapterTitle,
			selectedRecord.ChapterPage)

		///////////////////////////////////////////////////////
		manga := MangaResult{Title: selectedRecord.MangaTitle}
		chapter := Chapter{Number: selectedRecord.ChapterNumber, URL: selectedRecord.ChapterPage}
		mangaDir := filepath.Join(cacheDir, getModMangaTitle(selectedRecord.MangaTitle))
		pdfFilename := fmt.Sprintf("%s.pdf", selectedRecord.ChapterTitle)
		pdfPath := filepath.Join(mangaDir, pdfFilename)

		// Return if PDF already exists
		if _, err := os.Stat(pdfPath); err == nil {
			openPDF(pdfPath)
		} else {
			images, chapterTitle := scrapeChapterImages(selectedRecord.ChapterPage)
			pdfPath = downloadAndConvertToPDF(manga, chapter, images, chapterTitle)
		}

		/////////////////////////////////////////////////////////
		openPDF(pdfPath)

		// Update the ChapterPage URL for scraping further chapters
		selectedRecord.ChapterPage = strings.Join(strings.Split(selectedRecord.ChapterPage, "/")[:len(strings.Split(selectedRecord.ChapterPage, "/"))-1], "/")
		chapters := scrapeChapterList(selectedRecord.ChapterPage)

		if len(chapters) == 0 {
			fmt.Println("No chapters found. Exiting...")
			os.Exit(1)
		}

		inputControls(manga, chapters, chapter)
		///////////////////////////////////////////////////////
	}
}

func selectHistoryWithFzfNative(records []BrowseRecord) ([]int, error) {
	options := make([]string, len(records))
	for i, record := range records {
		options[i] = fmt.Sprintf("%s | %s", record.ChapterTitle, record.Timestamp.Format(time.RFC1123))
	}

	// Use native fzf
	cmd := exec.Command("fzf")
	cmd.Stdin = strings.NewReader(strings.Join(options, "\n"))
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	selectedOptions := strings.Split(strings.TrimSpace(out.String()), "\n")
	var selectedIndices []int
	for _, selected := range selectedOptions {
		for i, option := range options {
			if option == selected {
				selectedIndices = append(selectedIndices, i)
				break
			}
		}
	}

	return selectedIndices, nil
}

func selectHistoryWithGoFzf(records []BrowseRecord) ([]int, error) {
	// Create custom styles directly in the WithStyles function
	f, err := fzf.New(
		fzf.WithInputPosition("bottom"),
		fzf.WithSelectedPrefix("●"),
		fzf.WithUnselectedPrefix("◯"),
		fzf.WithStyles(
			fzf.WithStylePrompt(fzf.Style{
				ForegroundColor: "#FFFFFF", // White for the prompt
				BackgroundColor: "#1E1E1E", // Dark background
			}),
			fzf.WithStyleInputPlaceholder(fzf.Style{
				ForegroundColor: "#AAAAAA", // Light gray for input placeholder
			}),
			fzf.WithStyleInputText(fzf.Style{
				ForegroundColor: "#00FF00", // Green for input text
				BackgroundColor: "#1E1E1E", // Dark background for input text
			}),
			fzf.WithStyleCursorLine(fzf.Style{
				ForegroundColor: "#00FF00", // Green for input text
				BackgroundColor: "#1E1E1E", // Dark background for input text
			}),
			fzf.WithStyleCursor(fzf.Style{
				ForegroundColor: "#00ADD8", // Cyan for cursor
			}),
			fzf.WithStyleSelectedPrefix(fzf.Style{
				ForegroundColor: "#00ADD8", // Cyan for selected prefix
			}),
			fzf.WithStyleUnselectedPrefix(fzf.Style{
				ForegroundColor: "#FFFFFF", // White for unselected prefix
			}),
			fzf.WithStyleMatches(fzf.Style{
				ForegroundColor: "#00ADD8", // Cyan for matched characters
			}),
		),
	)
	if err != nil {
		return nil, err
	}

	options := make([]string, len(records))
	for i, record := range records {
		options[i] = fmt.Sprintf("%s | %s", record.ChapterTitle, record.Timestamp.Format(time.RFC1123))
	}

	idxs, err := f.Find(options, func(i int) string {
		return options[i]
	})
	if err != nil {
		return nil, err
	}

	return idxs, nil
}

func isFzfAvailable() bool {
	_, err := exec.LookPath("fzf")
	return err == nil
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

// Checks if "-jp" was passed in the command-line arguments
func checkWideSplitFlag() {
	for _, arg := range os.Args[1:] {
		if arg == "-ws" || arg == "--wide-split" {
			isWideSplitMode = true
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

func checkCacheDir() {

	tempDir := os.TempDir()
	cacheDir = filepath.Join(tempDir, ".cache", "goreadmanga")
}

func openDirectory(cacheDir string) error {
	// Just for future reference
	// OS PATHS
	// Config Paths
	// Linux
	// ${XDG_CONFIG_HOME:-${HOME}/.config}/example/config
	// MacOS
	// ${HOME}/Library/Application Support/example/config
	// Termux
	// $HOME/.config/example/config
	// Windows
	// %APPDATA%\example\config
	// %APPDATA%\.config\example\config

	// Get the operating system type
	absPath, _ := filepath.Abs(cacheDir) // Convert to absolute path
	switch runtime.GOOS {
	case "linux":
		// Use xdg-open for Linux
		return exec.Command("xdg-open", absPath).Start()
	case "android":
		// Use termux-open for Android (Termux)
		return exec.Command("termux-open", absPath).Start()
	case "darwin":
		// Use open for macOS
		return exec.Command("open", absPath).Start()
	case "windows":
		// Use explorer for Windows
		return exec.Command("explorer", absPath).Start()
	default:
		return fmt.Errorf("unsupported platform")
	}
}

func calculateStatistics(entries []BrowseRecord) {
	mangaStats := make(map[string]*MangaStatistics)
	for _, entry := range entries {
		baseURL := trimChapterFromURL(entry.ChapterPage)
		// Initialize statistics if it doesn't exist for this manga
		if _, exists := mangaStats[entry.MangaTitle]; !exists {
			chapters := scrapeChapterList(baseURL)
			mangaStats[entry.MangaTitle] = &MangaStatistics{
				TotalChapters:      len(chapters),
				ChaptersNotRead:    len(chapters), // Assume all chapters are initially unread
				UniqueChaptersRead: make(map[int]bool),
			}
		}

		stats := mangaStats[entry.MangaTitle]

		// Only count the chapter if it hasn't been read before
		if !stats.UniqueChaptersRead[entry.ChapterNumber] {
			stats.UniqueChaptersRead[entry.ChapterNumber] = true
			stats.ReadChapters++
			stats.ChaptersNotRead--
		}

		// Update the last read chapter
		stats.LastReadChapter = entry

		// Update timestamps
		timestamp := entry.Timestamp
		if stats.OldestTimestamp.IsZero() || timestamp.Before(stats.OldestTimestamp) {
			stats.OldestTimestamp = timestamp
		}
		if timestamp.After(stats.NewestTimestamp) {
			stats.NewestTimestamp = timestamp
		}

		stats.ReadingDates = append(stats.ReadingDates, timestamp)
	}

	// Aggregate statistics across all manga
	var totalChaptersRead, totalManga int
	var mostReadManga string
	mostReadCount := 0

	for title, stats := range mangaStats {
		percentage := (float64(stats.ReadChapters) / float64(stats.TotalChapters)) * 100

		// Print statistics with styles
		fmt.Println(blueFGpurpleBG.Render(fmt.Sprintf("Manga: %s | Read: %d/%d (%.2f%%)", title, stats.ReadChapters, stats.TotalChapters, percentage)))
		fmt.Println(yellowFGbrownBG.Render(fmt.Sprintf("  Last Read Chapter: %s", stats.LastReadChapter.ChapterTitle)))

		// Format timestamps
		oldestFormatted := stats.OldestTimestamp.Format("2 Jan 2006 [3:04 PM] GMT-07")
		newestFormatted := stats.NewestTimestamp.Format("2 Jan 2006 [3:04 PM] GMT-07")
		fmt.Println(redFGblackBG.Render(fmt.Sprintf("  Oldest Read Chapter: %s", oldestFormatted)))
		fmt.Println(redFGblackBG.Render(fmt.Sprintf("  Newest Read Chapter: %s", newestFormatted)))
		fmt.Println(redFGblackBG.Render(fmt.Sprintf("  Chapters Not Read: %d", stats.ChaptersNotRead)))

		// Update overall statistics
		totalChaptersRead += stats.ReadChapters
		totalManga++

		if stats.ReadChapters > mostReadCount {
			mostReadCount = stats.ReadChapters
			mostReadManga = title
		}
	}

	// Print overall statistics
	if totalManga > 0 {
		averageChapters := float64(totalChaptersRead) / float64(totalManga)
		fmt.Println(yellowFGbrownBG.Render(fmt.Sprintf("Average Chapters Read per Manga: %.2f", averageChapters)) + resetStyle.Render(""))
		fmt.Println(yellowFGbrownBG.Render(fmt.Sprintf("Most Read Manga: %s with %d chapters read.", mostReadManga, mostReadCount)) + resetStyle.Render(""))
	}

	// Calculate Reading Frequency and Streaks
	for title, stats := range mangaStats {
		if len(stats.ReadingDates) > 1 {
			uniqueDays := make(map[string]bool)
			streak := 0

			for _, date := range stats.ReadingDates {
				// Format the date to a string (YYYY-MM-DD) for uniqueness
				formattedDate := date.Format("2006-01-02")
				uniqueDays[formattedDate] = true
			}

			// Count unique days to determine the streak
			for i := range stats.ReadingDates {
				if uniqueDays[stats.ReadingDates[i].Format("2006-01-02")] {
					streak++
					delete(uniqueDays, stats.ReadingDates[i].Format("2006-01-02")) // Remove to ensure counting unique days only
				}
			}
			fmt.Println(fmt.Sprintf("Reading Streak for %s: %s days"+resetStyle.Render(""), blueFGpurpleBG.Render(title), lightMagentaWithBg.Render(fmt.Sprintf("%d", streak))))

		}
	}
}

// Function to remove the chapter part of the URL
func trimChapterFromURL(url string) string {
	// Find the index of the last '/'
	lastSlashIndex := strings.LastIndex(url, "/")
	if lastSlashIndex != -1 {
		// Return the URL up to and including the last '/'
		return url[:lastSlashIndex+1]
	}
	// Return the original URL if no '/' is found
	return url
}

func removeEmptyEntries(filePath string) error {
	// Read the existing data from the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error reading file %s: %v", filePath, err)
	}

	// Unmarshal the data into a slice of BrowseRecord
	var entries []BrowseRecord
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("error unmarshaling data from %s: %v", filePath, err)
	}

	// Filter entries to keep only those with non-empty chapter_page and chapter_title
	filteredEntries := make([]BrowseRecord, 0, len(entries))

	for _, entry := range entries {
		if entry.ChapterPage != "" || entry.ChapterTitle != "" {
			filteredEntries = append(filteredEntries, entry)
		}
	}

	// Check if any entries were removed
	if len(filteredEntries) != len(entries) {
		fmt.Printf("Removed %d entries with empty chapter_page or chapter_title from %s\n", len(entries)-len(filteredEntries), filePath)
	} else {
		fmt.Println("No entries removed from", filePath)
	}

	// Marshal the filtered entries back to JSON
	filteredData, err := json.MarshalIndent(filteredEntries, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling filtered data: %v", err)
	}

	// Write the updated data back to the file
	err = os.WriteFile(filePath, filteredData, 0644) // Adjust permissions as needed
	if err != nil {
		return fmt.Errorf("error writing updated data to %s: %v", filePath, err)
	}

	return nil
}
