/*
Copyright © 2021 Stamus Networks oss@stamus-networks.com

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/
package extract

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type ErrOutOfFiles struct {
}

func (e ErrOutOfFiles) Error() string {
	return "No more files"
}

type PcapFileList struct {
	Files          []string
	DirName        string
	FileName       string
	Index          int
	FileParsing    *regexp.Regexp
	ThreadIndex    int
	TimestampIndex int
}

func NewPcapFileList(dname string, event Event, fileFormat string) *PcapFileList {
	pl := new(PcapFileList)
	pl.DirName = dname
	pl.buildPcapNameParsing(fileFormat)
	if len(event.CaptureFile) > 0 {
		fullName := path.Join(dname, event.CaptureFile)
		pl.Files = append(pl.Files, fullName)
		err := pl.buildPcapList()
		if err != nil {
			return nil
		}
		pl.FileName = event.CaptureFile
	} else {
		logrus.Debug("Scanning will start soon")
		err := pl.buildFullPcapList()
		if err != nil {
			return nil
		}
	}
	return pl
}

/* Suricata supports following expansion
- %n -- thread number
- %i -- thread id
- %t -- timestamp
*/
func (pl *PcapFileList) buildPcapNameParsing(fileFormat string) {
	/* get each token */
	threadIndex := strings.Index(fileFormat, "%n")
	if threadIndex == -1 {
		threadIndex = strings.Index(fileFormat, "%i")
	}
	timestampIndex := strings.Index(fileFormat, "%t")
	if threadIndex < timestampIndex {
		pl.ThreadIndex = 1
		pl.TimestampIndex = 2
	} else {
		pl.ThreadIndex = 2
		pl.TimestampIndex = 1
	}
	/* handle the case where just the timestamp is available */
	if threadIndex == -1 {
		pl.ThreadIndex = -1
		pl.TimestampIndex = 1
	}
	regexpString := strings.Replace(fileFormat, "%n", `(\d+)`, 1)
	regexpString = strings.Replace(regexpString, "%i", `(\d+)`, 1)
	regexpString = strings.Replace(regexpString, "%t", `(\d+)`, 1)
	logrus.Debug("Using regexp: ", regexpString)
	/* build regular expression */
	pl.FileParsing = regexp.MustCompile(regexpString)
}

func (pl *PcapFileList) GetNext() (string, error) {
	if pl.Index < len(pl.Files) {
		pfile := pl.Files[pl.Index]
		pl.Index += 1
		return pfile, nil
	}
	return "", &ErrOutOfFiles{}
}

func (pl *PcapFileList) buildPcapList() error {
	if pl.Files == nil || len(pl.Files) < 1 {
		return errors.New("No file available")
	}
	dName := path.Dir(pl.Files[0])
	logrus.Debug("Scanning directory: ", dName)
	filePart := path.Base(pl.Files[0])
	match := pl.FileParsing.FindStringSubmatch(filePart)
	if len(match) == 0 {
		logrus.Errorf("file %s does not match file format", filePart)
		return errors.New("Invalid file name in event")
	}
	var threadIndex int64 = -1
	if pl.ThreadIndex != -1 {
		var err error
		threadIndex, err = strconv.ParseInt(match[pl.ThreadIndex], 10, 64)
		if err != nil {
			logrus.Warning("Can't parse integer")
		}
	}
	timestampValue, err := strconv.ParseInt(match[pl.TimestampIndex], 10, 64)
	timestamp := time.Unix(timestampValue, 0)
	if err != nil {
		logrus.Warning("Can't parse integer")
		return errors.New("Invalid timestamp in file name")
	}
	files, err := ioutil.ReadDir(dName)
	if err != nil {
		logrus.Warningf("Can't open directory %s: %s", dName, err)
	}
	for _, file := range files {
		if file.Name() == filePart {
			continue
		}
		lMatch := pl.FileParsing.FindStringSubmatch(file.Name())
		if lMatch == nil {
			continue
		}
		if pl.ThreadIndex != -1 {
			lThreadIndex, err := strconv.ParseInt(lMatch[pl.ThreadIndex], 10, 64)
			if err != nil {
				logrus.Warning("Can't parse integer")
				continue
			}
			if lThreadIndex != threadIndex {
				continue
			}
		}
		lTimestampValue, err := strconv.ParseInt(lMatch[pl.TimestampIndex], 10, 64)
		lTimestamp := time.Unix(lTimestampValue, 0)
		if err != nil {
			logrus.Warning("Can't parse integer")
			continue
		}
		if timestamp.Before(lTimestamp) {
			logrus.Infof("Adding file %s", file.Name())
			pl.Files = append(pl.Files, path.Join(dName, file.Name()))
		} else {
			logrus.Debugf("Skipping file %s", file.Name())
		}
	}
	return nil
}

func (pl *PcapFileList) buildFullPcapList() error {
	logrus.Debugf("Scanning directory: %s", pl.DirName)
	files, err := ioutil.ReadDir(pl.DirName)
	if err != nil {
		return fmt.Errorf("Can't open directory %s: %s", pl.DirName, err)
	}
	for _, file := range files {
		lMatch := pl.FileParsing.FindStringSubmatch(file.Name())
		if lMatch == nil {
			continue
		}
		logrus.Infof("Adding file %s", file.Name())
		pl.Files = append(pl.Files, path.Join(pl.DirName, file.Name()))
	}
	return nil
}
