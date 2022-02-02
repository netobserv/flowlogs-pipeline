/*
 * Copyright (C) 2021 IBM, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package location

import (
	"archive/zip"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/ip2location/ip2location-go/v9"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Info struct {
	CountryName     string `json:"country_name"`
	CountryLongName string `json:"country_long"`
	RegionName      string `json:"region_name"`
	CityName        string `json:"city_name"`
	Latitude        string `json:"latitude"`
	Longitude       string `json:"longitude"`
}

const (
	DBFilename        = "IP2LOCATION-LITE-DB9.BIN"
	DBFileLocation    = "/tmp/location_db.bin"
	DBZIPFileLocation = "/tmp/location_db.bin" + ".zip"
	// REF: Original location from ip2location DB is: "https://www.ip2location.com/download/?token=OpOljbgT6K2WJnFrFBBmBzRVNpHlcYqNN4CMeGavvh0pPOpyu16gKQyqvDMxTDF4&file=DB9LITEBIN"
	DbUrl = "https://raw.githubusercontent.com/netobserv/flowlogs2metrics/main/contrib/location/location.db"
)

var locationDB *ip2location.DB

type OSIO struct {
	Stat     func(string) (os.FileInfo, error)
	Create   func(string) (*os.File, error)
	MkdirAll func(string, os.FileMode) error
	OpenFile func(string, int, os.FileMode) (*os.File, error)
	Copy     func(io.Writer, io.Reader) (int64, error)
}

var _osio = OSIO{}
var _dbURL string

func init() {
	_osio.Stat = os.Stat
	_osio.Create = os.Create
	_osio.MkdirAll = os.MkdirAll
	_osio.OpenFile = os.OpenFile
	_osio.Copy = io.Copy
	_dbURL = DbUrl
}

func InitLocationDB() error {

	if _, err := _osio.Stat(DBFileLocation); errors.Is(err, os.ErrNotExist) {
		log.Infof("Downloading location DB into local file %s ", DBFileLocation)
		out, err := _osio.Create(DBZIPFileLocation)
		if err != nil {
			return fmt.Errorf("failed os.Create %v ", err)
		}

		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: tr}
		resp, err := client.Get(_dbURL)
		if err != nil {
			return fmt.Errorf("failed http.Get %v ", err)
		}

		_, err = io.Copy(out, resp.Body)
		if err != nil {
			return fmt.Errorf("failed io.Copy %v ", err)
		}

		err = resp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed resp.Body.Close %v ", err)
		}

		err = out.Close()
		if err != nil {
			return fmt.Errorf("failed out.Close %v ", err)
		}

		err = unzip(DBZIPFileLocation, DBFileLocation)
		if err != nil {
			return fmt.Errorf("failed unzip %v ", err)
		}

		log.Infof("Download completed succeful")
	}

	log.Debugf("Loading location DB")
	db, err := ip2location.OpenDB(DBFileLocation + "/" + DBFilename)
	if err != nil {
		return fmt.Errorf("OpenDB err - %v ", err)
	}

	locationDB = db
	return nil
}

func GetLocation(ip string) (error, *Info) {

	if locationDB == nil {
		return fmt.Errorf("no location DB available"), nil
	}

	res, err := locationDB.Get_all(ip)
	if err != nil {
		return err, nil
	}

	return nil, &Info{
		CountryName:     res.Country_short,
		CountryLongName: res.Country_long,
		RegionName:      res.Region,
		CityName:        res.City,
		Latitude:        fmt.Sprintf("%f", res.Latitude),
		Longitude:       fmt.Sprintf("%f", res.Longitude),
	}
}

//goland:noinspection ALL
func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		filePath := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			err = _osio.MkdirAll(filePath, f.Mode())
			if err != nil {
				log.Error(err)
				return err
			}
		} else {
			var fileDir string
			if lastIndex := strings.LastIndex(filePath, string(os.PathSeparator)); lastIndex > -1 {
				fileDir = filePath[:lastIndex]
			}

			err = _osio.MkdirAll(fileDir, f.Mode())
			if err != nil {
				log.Error(err)
				return err
			}
			df, err := _osio.OpenFile(
				filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer df.Close()

			_, err = _osio.Copy(df, rc)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
