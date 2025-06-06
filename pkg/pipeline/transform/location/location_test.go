/*
 * Copyright (C) 2022 IBM, Inc.
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
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/ip2location/ip2location-go/v9"
	"github.com/stretchr/testify/require"
)

func Test_InitLocationDB(t *testing.T) {
	// fail in os.Create
	_osio.Stat = func(_ string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	_osio.Create = func(_ string) (*os.File, error) { return nil, fmt.Errorf("test") }
	err := InitLocationDB()
	require.Contains(t, err.Error(), "os.Create")
	_osio.Stat = os.Stat
	_osio.Create = os.Create

	// fail in http.Get
	_osio.Stat = func(_ string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	_osio.Create = func(_ string) (*os.File, error) { return nil, nil }
	_dbURL = "test_fake"
	err = InitLocationDB()
	require.Contains(t, err.Error(), "http.Get")
	_dbURL = dbURL
	_osio.Stat = os.Stat
	_osio.Create = os.Create

	// fail in io.Copy
	_osio.Stat = func(_ string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	_osio.Create = func(_ string) (*os.File, error) { return nil, nil }
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
		_, _ = res.Write([]byte("test"))
	}))
	_dbURL = testServer.URL
	err = InitLocationDB()
	require.Contains(t, err.Error(), "io.Copy")
	testServer.Close()
	_dbURL = dbURL
	_osio.Stat = os.Stat
	_osio.Create = os.Create

	// fail again in io.Copy
	_osio.Stat = func(_ string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	_osio.Create = func(_ string) (*os.File, error) { return nil, nil }
	testServer = httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
		res.WriteHeader(http.StatusOK)
	}))
	_dbURL = testServer.URL
	err = InitLocationDB()
	require.Contains(t, err.Error(), "io.Copy")
	testServer.Close()
	_dbURL = dbURL
	_osio.Stat = os.Stat
	_osio.Create = os.Create

	// fail in unzip
	_osio.Stat = func(_ string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	testServer = httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
		_, _ = res.Write([]byte("test"))
		res.WriteHeader(http.StatusOK)
	}))
	_dbURL = testServer.URL
	err = InitLocationDB()
	require.Contains(t, err.Error(), "failed unzip")
	testServer.Close()
	_dbURL = dbURL
	_osio.Stat = os.Stat

	// fail in OpenDB
	_osio.Stat = func(_ string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)
	_, _ = zipWriter.Create(dbFilename)
	zipWriter.Close()
	testServer = httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
		_, _ = res.Write(buf.Bytes())
		res.WriteHeader(http.StatusOK)
	}))
	_dbURL = testServer.URL
	err = InitLocationDB()
	require.Error(t, err)
	testServer.Close()
	_dbURL = dbURL
	_osio.Stat = os.Stat
	// success
	// NOTE:: Downloading the DB is a long operation, about 30 seconds, and this delays the tests
	// TODO: Consider remove this test
	os.RemoveAll(dbFileLocation)
	initLocationDBErr := InitLocationDB()
	require.Nil(t, initLocationDBErr)
}

func Test_GetLocation(t *testing.T) {
	locationDB = nil
	info, err := GetLocation("test")
	require.Contains(t, err.Error(), "no location DB available")
	require.Nil(t, info)

	locationDB = &ip2location.DB{}
	info, err = GetLocation("test")
	require.Contains(t, info.CountryName, "Invalid database file.")
	require.Nil(t, err)
}

func Test_unzip(t *testing.T) {
	// success
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)
	_, _ = zipWriter.Create("test_file_in_zip")
	zipWriter.Close()
	_ = os.WriteFile("/tmp/test_zip.zip", buf.Bytes(), 0777)
	err := unzip("/tmp/test_zip.zip", "/tmp/")
	require.Nil(t, err)

	// failed unzip
	err = unzip("fake_test", "fake_test")
	require.Error(t, err)

	// failed os.MkdirAll
	_osio.MkdirAll = func(string, os.FileMode) error { return fmt.Errorf("test") }
	err = unzip("/tmp/test_zip.zip", "/tmp/")
	require.Error(t, err)
	_osio.MkdirAll = os.MkdirAll

	// failed os.OpenFile
	_osio.OpenFile = func(string, int, os.FileMode) (*os.File, error) { return nil, fmt.Errorf("test") }
	err = unzip("/tmp/test_zip.zip", "/tmp/")
	require.Error(t, err)
	_osio.OpenFile = os.OpenFile

	// failed io.Copy
	_osio.Copy = func(io.Writer, io.Reader) (int64, error) { return 0, fmt.Errorf("test") }
	err = unzip("/tmp/test_zip.zip", "/tmp/")
	require.Error(t, err)
	_osio.Copy = io.Copy

	// failed os.MkdirAll dir
	_osio.MkdirAll = func(string, os.FileMode) error { return fmt.Errorf("test") }
	buf = new(bytes.Buffer)
	zipWriter = zip.NewWriter(buf)
	_, _ = zipWriter.Create("new_dir/")
	zipWriter.Close()
	_ = os.WriteFile("/tmp/test_zip.zip", buf.Bytes(), 0777)
	err = unzip("/tmp/test_zip.zip", "/tmp/")
	require.Error(t, err)
	_osio.MkdirAll = os.MkdirAll
}
