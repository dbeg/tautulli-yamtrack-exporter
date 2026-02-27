package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

type TautulliResponse struct {
	Response struct {
		Result  string `json:"result"`
		Message string `json:"message"`
		Data    struct {
			Data            []HistoryItem `json:"data"`
			RecordsFiltered int           `json:"recordsFiltered"`
			RecordsTotal    int           `json:"recordsTotal"`
		} `json:"data"`
	} `json:"response"`
}

type HistoryItem struct {
	Title            string      `json:"title"`
	MediaType        string      `json:"media_type"`
	Started          int64       `json:"started"`
	Stopped          int64       `json:"stopped"`
	Date             int64       `json:"date"`
	RatingKey        int         `json:"rating_key"`
	ParentTitle      string      `json:"parent_title"`
	GrandparentTitle string      `json:"grandparent_title"`
	MediaIndex       interface{} `json:"media_index"`
	ParentMediaIndex interface{} `json:"parent_media_index"`
}

var verbose bool
var sessionID string
var createdFiles []string

func logVerbose(format string, a ...interface{}) {
	if verbose {
		fmt.Printf("[DEBUG] "+format+"\n", a...)
	}
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return strconv.Itoa(int(val))
	case int:
		return strconv.Itoa(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func formatISO8601(ts int64) string {
	if ts == 0 {
		return ""
	}
	// Yamtrack wants: 2023-01-16 03:56:13+00:00
	return time.Unix(ts, 0).Format("2006-01-02 15:04:05-07:00")
}

func main() {
	var (
		endpoint, apiKey, username, startDateStr, endDateStr string
		dryRun                                               bool
	)

	flag.StringVar(&endpoint, "endpoint", "http://127.0.0.1:8181", "Tautulli URL")
	flag.StringVar(&apiKey, "api-key", "", "Tautulli API Key")
	flag.StringVar(&username, "username", "", "Tautulli Username")
	flag.StringVar(&startDateStr, "start", "", "Filter after (YYYY-MM-DD)")
	flag.StringVar(&endDateStr, "end", "", "Filter before (YYYY-MM-DD)")
	flag.BoolVar(&dryRun, "dry-run", false, "Count records without downloading")
	flag.BoolVar(&verbose, "verbose", false, "Enable detailed logging")
	flag.Parse()

	if apiKey == "" || username == "" {
		fmt.Println("Error: --api-key and --username are required.")
		os.Exit(1)
	}

	sessionID = time.Now().Format("2006-01-02_15-04-05")

	var after, before int64
	if startDateStr != "" {
		t, _ := time.Parse("2006-01-02", startDateStr)
		after = t.Unix()
	}
	if endDateStr != "" {
		t, _ := time.Parse("2006-01-02", endDateStr)
		before = t.Unix()
	}

	categorizedData := make(map[string][]HistoryItem)
	var globalHistory []HistoryItem
	mediaTypes := []string{"movie", "episode"}

	fmt.Println("Exporting data from Tautulli server: " + endpoint + " for user: " + username)

	for _, mType := range mediaTypes {
		fmt.Printf("\n>>> Category: %s\n", mType)
		startOffset := 0
		pageSize := 100

		for {
			u, _ := url.Parse(endpoint + "/api/v2")
			q := u.Query()
			q.Set("apikey", apiKey)
			q.Set("cmd", "get_history")
			q.Set("user", username)
			q.Set("media_type", mType)
			q.Set("start", strconv.Itoa(startOffset))
			q.Set("length", strconv.Itoa(pageSize))
			if after > 0 {
				q.Set("after", strconv.FormatInt(after, 10))
			}
			if before > 0 {
				q.Set("before", strconv.FormatInt(before, 10))
			}
			u.RawQuery = q.Encode()

			logVerbose("Fetching: %s", u.String())

			resp, err := http.Get(u.String())
			if err != nil {
				break
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			var tResp TautulliResponse
			json.Unmarshal(body, &tResp)

			total := tResp.Response.Data.RecordsFiltered
			if dryRun {
				fmt.Printf("    Dry run found %d records.\n", total)
				break
			}
			if total == 0 {
				break
			}

			categoryDir := filepath.Join("tmp", sessionID, mType)
			os.MkdirAll(categoryDir, 0755)
			batchPath := filepath.Join(categoryDir, fmt.Sprintf("batch_%d.json", startOffset))
			os.WriteFile(batchPath, body, 0644)
			createdFiles = append(createdFiles, batchPath)

			items := tResp.Response.Data.Data
			categorizedData[mType] = append(categorizedData[mType], items...)
			globalHistory = append(globalHistory, items...)

			fmt.Printf("\r    Progress: %d / %d", len(categorizedData[mType]), total)
			if len(categorizedData[mType]) >= total || len(items) < pageSize {
				fmt.Println()
				break
			}
			startOffset += pageSize
		}

		if !dryRun && len(categorizedData[mType]) > 0 {

			fileName := fmt.Sprintf("./out/tautulli-yamtrack-exporter_%s_%s.csv", sessionID, mType)
			writeToCSV(fileName, categorizedData[mType])
		}
	}

	if !dryRun && len(globalHistory) > 0 {
		writeToCSV(fmt.Sprintf("./out/tautulli-yamtrack-exporter_%s_global.csv", sessionID), globalHistory)
		printSummary()
	}
}

func writeToCSV(filename string, items []HistoryItem) {
	if len(items) == 0 {
		return
	}

	dedupedMap := make(map[int]HistoryItem)
	for _, item := range items {
		existing, found := dedupedMap[item.RatingKey]
		currentTs := item.Stopped
		if currentTs == 0 {
			currentTs = item.Date
		}
		existingTs := existing.Stopped
		if existingTs == 0 {
			existingTs = existing.Date
		}

		if !found || currentTs > existingTs {
			dedupedMap[item.RatingKey] = item
		}
	}

	var sortedItems []HistoryItem
	for _, item := range dedupedMap {
		sortedItems = append(sortedItems, item)
	}
	sort.Slice(sortedItems, func(i, j int) bool {
		tsI := sortedItems[i].Stopped
		if tsI == 0 {
			tsI = sortedItems[i].Date
		}
		tsJ := sortedItems[j].Stopped
		if tsJ == 0 {
			tsJ = sortedItems[j].Date
		}
		return tsI > tsJ
	})

	file, err := os.Create(filename)
	if err != nil {
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.UseCRLF = true
	defer writer.Flush()

	writer.Write([]string{
		"media_id",
		"source",
		"media_type",
		"title",
		"image",
		"season_number",
		"episode_number",
		"score",
		"status",
		"notes",
		"start_date",
		"end_date",
		"progress",
	})

	for _, item := range sortedItems {
		displayTitle := item.Title
		seasonNum := ""
		episodeNum := ""

		if item.MediaType == "episode" {
			if item.GrandparentTitle != "" {
				displayTitle = item.GrandparentTitle
			}
			seasonNum = toString(item.ParentMediaIndex)
			episodeNum = toString(item.MediaIndex)
		}

		writer.Write([]string{
			"",
			"tmdb",
			item.MediaType,
			displayTitle,
			"",
			seasonNum,
			episodeNum,
			"",
			"Completed",
			"Exported from Tautulli via github.com/dbeg/tautulli-yamtrack-exporter",
			formatISO8601(item.Started),
			formatISO8601(item.Stopped),
			"100",
		})
	}
	absPath, _ := filepath.Abs(filename)
	createdFiles = append(createdFiles, absPath)
}

func printSummary() {
	fmt.Println("\n--- EXPORT SUMMARY ---")

	fmt.Printf("Session ID: %s\n", sessionID)

	fmt.Println("Temp API response data:")
	if verbose {
		for _, f := range createdFiles {
			if filepath.Ext(f) == ".json" {
				fmt.Printf(" - %s\n", f)
			}
		}
	}

	fmt.Println("Main CSVs created:")
	for _, f := range createdFiles {
		if filepath.Ext(f) == ".csv" {
			fmt.Printf(" - %s\n", f)
		}
	}

	fmt.Println("----------------------")
}
