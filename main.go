package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"golang.org/x/net/proxy"
)

var (
	appLogger    *log.Logger
	reportLogger *log.Logger
	// Tor Proxy ayarları
	proxyIP   = "127.0.0.1"
	proxyPort = "9150"
	proxyAddr = proxyIP + ":" + proxyPort
)

func initSystem() {

	if _, err := os.Stat("logs"); os.IsNotExist(err) {
		os.Mkdir("logs", 0755)
	}
	if _, err := os.Stat("screenshots"); os.IsNotExist(err) {
		os.Mkdir("screenshots", 0755)
	}

	appFile, _ := os.OpenFile("logs/app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	appLogger = log.New(appFile, "", log.LstdFlags)

	reportFile, _ := os.OpenFile("logs/scan_report.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	reportLogger = log.New(reportFile, "", log.LstdFlags)
}

func printInfo(msg string) {
	fmt.Printf(" i %s\n", msg)
	appLogger.Printf("[INFO] ", msg)
}

func printSuccess(msg string) {
	fmt.Printf(" ✓ %s\n", msg)
	appLogger.Printf("[SUCCESS] %s", msg)
}

func printHeader() {

	fmt.Println("\n--- Dark Web Forum Scraper ---\n")
}

func checkTorConnection() {
	printInfo(fmt.Sprintf("Tor soket bağlantısı kontrol ediliyor (%s)", proxyPort))

	conn, err := net.DialTimeout("tcp", proxyAddr, 2*time.Second)
	if err != nil {
		fmt.Printf(" X Tor Proxy hatası: %v\n", err)
		os.Exit(1)
	}
	conn.Close()
	printSuccess("Yerel Tor portuna erişildi.")

	printInfo("Tor Project üzerinden resmi doğrulama yapılıyor...")

	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		log.Fatalf("SOCKS5 hatası: %v", err)
	}

	client := &http.Client{
		Transport: &http.Transport{Dial: dialer.Dial},
		Timeout:   20 * time.Second,
	}

	resp, err := client.Get("https://check.torproject.org/")
	if err != nil {
		printInfo("Tor Project sitesine ulaşılamadı (Timeout), ama proxy açık.")
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyString := string(bodyBytes)

	if strings.Contains(bodyString, "Congratulations. This browser is configured to use Tor.") {
		printSuccess("TOR PROJECT ONAYI: 'Congratulations. This browser is configured to use Tor.' ")
	} else {
		printInfo("Tor bağlantısı var ama 'Congratulations' mesajı yakalanamadı.")
	}
}

func readTargets(filename string) ([]string, error) {
	var targets []string
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			targets = append(targets, line)
		}
	}
	return targets, scanner.Err()
}

func takeScreenshot(url string) error {

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ProxyServer("socks5://"+proxyAddr),
		chromedp.WindowSize(1920, 1080),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAlloc()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	var imageBuf []byte

	printInfo("Navigating to: " + url)
	printInfo("Waiting for page load...")

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(15*time.Second),
		chromedp.CaptureScreenshot(&imageBuf),
	)

	if err != nil {
		return err
	}

	clearName := strings.ReplaceAll(strings.TrimPrefix(url, "http://"), "/", "_")
	if len(clearName) > 50 {
		clearName = clearName[:50]
	}

	fileName := filepath.Join("screenshots", fmt.Sprintf("screenshot_%s.png", clearName))

	if err := os.WriteFile(fileName, imageBuf, 0644); err != nil {
		return err
	}

	msg := fmt.Sprintf("Screenshot saved: %s", fileName)
	printSuccess(msg)
	reportLogger.Printf("SUCCESS -> Screenshot saved for %s at %s", url, fileName)

	return nil
}

func processURL(url string, index int) {
	printInfo(fmt.Sprintf("Processing Target %d: %s", index+1, url))

	err := takeScreenshot(url)
	if err != nil {
		fmt.Printf(" X Failed: %v\n", err)
		appLogger.Printf("[ERROR] Failed to screenshot %s: %v", url, err)
		reportLogger.Printf("FAILED -> %s Error: %v", url, err)
	}
}

func main() {
	initSystem()
	checkTorConnection()

	printInfo("Scan raporu oluşturuluyor...")
	printSuccess("Scan raporu oluşturuldu: logs/scan_report.log")

	targets, err := readTargets("targets.yaml")
	if err != nil {
		printInfo("targets.yaml bulunamadı.")
	}

	for {
		printHeader()

		for i, t := range targets {
			fmt.Printf(" %d. %s\n", i+1, t)
		}

		fmt.Printf(" %d. Scrape all forums (Screenshot Mode)\n", len(targets)+1)
		fmt.Printf(" 0. Exit\n\n")

		fmt.Print("Select an option: ")

		var choice int
		fmt.Scan(&choice)

		if choice == 0 {
			printInfo("Exiting...")
			break
		}

		if choice == len(targets)+1 {
			for i, t := range targets {
				processURL(t, i)
				time.Sleep(2 * time.Second)
			}
			fmt.Println("\nPress Enter to continue...")
			var tmp string
			fmt.Scanln(&tmp)
		} else if choice > 0 && choice <= len(targets) {
			processURL(targets[choice-1], choice-1)

			fmt.Println("\nPress Enter to continue...")
			bufio.NewReader(os.Stdin).ReadBytes('\n')
			bufio.NewReader(os.Stdin).ReadBytes('\n')
		} else {
			fmt.Println("Invalid option!")
			time.Sleep(1 * time.Second)
		}
	}
}

/*
--- PROJE YAPIM AŞAMASINDA KULLANILAN KAYNAKLAR ---

https://pkg.go.dev/std
https://pkg.go.dev/bufio
https://pkg.go.dev/context
https://pkg.go.dev/fmt
https://pkg.go.dev/io
https://pkg.go.dev/log
https://pkg.go.dev/net
https://pkg.go.dev/net/http
https://pkg.go.dev/os
https://pkg.go.dev/path/filepath
https://pkg.go.dev/strings
https://pkg.go.dev/time
https://pkg.go.dev/github.com/chromedp/chromedp
https://github.com/chromedp/chromedp
https://pkg.go.dev/golang.org/x/net/proxy
https://github.com/golang/net
https://go.dev/doc/
https://go.dev/tour/
https://gobyexample.com/
https://pkg.go.dev/
https://forum.golangbridge.org/t/use-of-x-net-proxy-and-fromenvironmentusing-dialer-does-not-work/33044

*/
