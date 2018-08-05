package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
)

// Movie represents a movie from yts
type Movie struct {
	ID                      int      `json:"id"`
	URL                     string   `json:"url"`
	ImdbCode                string   `json:"imdb_code"`
	Title                   string   `json:"title"`
	TitleEnglish            string   `json:"title_english"`
	TitleLong               string   `json:"title_long"`
	Slug                    string   `json:"slug"`
	Year                    int      `json:"year"`
	Rating                  float64  `json:"rating"`
	Runtime                 int      `json:"runtime"`
	Genres                  []string `json:"genres"`
	DownloadCount           int      `json:"download_count"`
	LikeCount               int      `json:"like_count"`
	DescriptionIntro        string   `json:"description_intro"`
	DescriptionFull         string   `json:"description_full"`
	YtTrailerCode           string   `json:"yt_trailer_code"`
	Language                string   `json:"language"`
	MpaRating               string   `json:"mpa_rating"`
	BackgroundImage         string   `json:"background_image"`
	BackgroundImageOriginal string   `json:"background_image_original"`
	SmallCoverImage         string   `json:"small_cover_image"`
	MediumCoverImage        string   `json:"medium_cover_image"`
	LargeCoverImage         string   `json:"large_cover_image"`
	Torrents                []struct {
		URL              string `json:"url"`
		Hash             string `json:"hash"`
		Quality          string `json:"quality"`
		Seeds            int    `json:"seeds"`
		Peers            int    `json:"peers"`
		Size             string `json:"size"`
		SizeBytes        int    `json:"size_bytes"`
		DateUploaded     string `json:"date_uploaded"`
		DateUploadedUnix int    `json:"date_uploaded_unix"`
	} `json:"torrents"`
	DateUploaded     string `json:"date_uploaded"`
	DateUploadedUnix int    `json:"date_uploaded_unix"`
}

// Payload is the api payload
type Payload struct {
	Data struct {
		MovieCount int     `json:"movie_count"`
		Limit      int     `json:"limit"`
		PageNumber int     `json:"page_number"`
		Movies     []Movie `json:"movies"`
	} `json:"data"`
}

// APIURL is the url for API
const APIURL = "https://yts.am/api/v2/list_movies.json?limit=50&page=%d"

func main() {
	if len(os.Args) != 2 {
		errExit("USAGE: %s FILE", os.Args[0])
	}
	var movies []Movie

	f, err := os.OpenFile(os.Args[1], os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		errExit("unable to open file: %v", err)
	}

	err = json.NewDecoder(f).Decode(&movies)
	if err != nil {
		if s, err := f.Stat(); err != nil && s.Size() > 0 {
			errExit("unable to read file: %v", err)
		}
	}

	j := 2
	cl := len(movies)
	ttd := cl
	for i := 1; i < j; i++ {
		resp, err := http.DefaultClient.Get(fmt.Sprintf(APIURL, i))
		if err != nil {
			perr("error unable to response: %v", err)
			continue
		}

		payload := new(Payload)
		err = json.NewDecoder(resp.Body).Decode(payload)
		if err != nil {
			perr("unable to decode JSON: %v", err)
			continue
		}

		tl := payload.Data.MovieCount
		dl := tl - cl
		ml := len(payload.Data.Movies)
		index := int(math.Min(float64(ml), float64(dl)))
		j = int(math.Ceil(float64(dl)/50.0)) + 1
		ttd += index

		movies = append(movies, payload.Data.Movies[:index]...)
		fmt.Printf("Page: %03d of %03d, Total Movies: %06d, Movies: %06d\r\n", i, j, tl, ttd)
	}

	f.Truncate(0)
	f.Seek(0, 0)
	err = json.NewEncoder(f).Encode(movies)
	if err != nil {
		perr("unable to encode JSON: %v", err)
		return
	}

	for _, movie := range movies {
		downloadRes(movie)
	}

}

func errExit(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\r\n", args...)
	os.Exit(1)
}

func perr(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\r\n", args...)
}

func download(link string) error {
	p := strings.Replace(link, "https://yts.am/", "", 1)
	dir := path.Dir(p)
	ext := path.Ext(p)
	if ext == "" {
		ext = ".torrent"
	} else {
		ext = ""
	}

	p = p + ext

	if _, err := os.Stat(p); !os.IsNotExist(err) {
		perr("file already exists: %v", p)
		return err
	}

	err := os.MkdirAll(dir, 644)
	if err != nil {
		perr("unable to create directory: %v", err)
		return err
	}

	resp, err := http.DefaultClient.Get(link)
	if err != nil {
		return err
	}

	f, err := os.Create(p)
	if err != nil {
		perr("unable to create file: %v", err)
		return err
	}

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		perr("unable to download file: %v", err)
		return err
	}
	return nil
}

func downloadRes(movie Movie) {
	var wg sync.WaitGroup
	assets := downloadables(movie)
	wg.Add(len(assets))
	for _, res := range assets {
		go func(l string) {
			err := download(l)
			if err != nil {
				perr("unable to download asset: %v", err)
			}
			wg.Done()
		}(res)
	}
	wg.Wait()
}

func downloadables(movie Movie) []string {
	res := make([]string, len(movie.Torrents)+5)
	res[0] = movie.BackgroundImage
	res[1] = movie.BackgroundImageOriginal
	res[2] = movie.SmallCoverImage
	res[3] = movie.MediumCoverImage
	res[4] = movie.LargeCoverImage
	for i, t := range movie.Torrents {
		res[5+i] = t.URL
	}
	return res
}
