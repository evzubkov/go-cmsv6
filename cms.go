package cmsv6

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	urlLib "net/url"

	"github.com/spf13/cast"
	"golang.org/x/net/context"
)

type Client struct {
	login        string
	password     string
	gateway      string
	apiPort      uint
	downloadPort uint
	jsession     string
}

type ClientDeps struct {
	Login        string
	Password     string
	Gateway      string
	ApiPort      uint
	DownloadPort uint
}

func NewClient(deps *ClientDeps) *Client {
	return &Client{login: deps.Login,
		password:     deps.Password,
		gateway:      deps.Gateway,
		apiPort:      deps.ApiPort,
		downloadPort: deps.DownloadPort}
}

type checkSessionResponser struct {
	Result   uint   `json:"result"`
	Jsession string `json:"jsession"`
}

func (c *Client) checkSession(ctx context.Context) error {
	if c.jsession == "" {
		var answer checkSessionResponser

		req, _ := http.NewRequestWithContext(
			ctx, "POST",
			fmt.Sprintf("http://%s:%d/StandardApiAction_login.action?account=%s&password=%s",
				c.gateway,
				c.apiPort,
				c.login,
				c.password),
			nil,
		)

		httpClient := http.Client{}
		resp, err := httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		if resp.StatusCode != 200 {
			err = fmt.Errorf("fail to send request to server. Status code: %d, info: %+v", resp.StatusCode, cast.ToString(body))
			return err
		}

		if err = json.Unmarshal(body, &answer); err != nil {
			return err
		}

		if answer.Result != 0 {
			err = fmt.Errorf("fail auth. Result: %+v", answer)
			return err
		}

		c.jsession = answer.Jsession
	}

	return nil
}

type GetVideoFileInfoResponse struct {
	Files []struct {
		DownTaskUrl string `json:"DownTaskUrl"`
	}
}

func (c *Client) GetVideoFileInfo(ctx context.Context, startDateTime, stopDateTime time.Time, deviceId, channel uint) (result GetVideoFileInfoResponse, err error) {
	if err = c.checkSession(ctx); err != nil {
		return
	}

	year := startDateTime.Year()
	month := startDateTime.Month()
	day := startDateTime.Day()

	startSecond := startDateTime.Hour()*3600 + startDateTime.Minute()*60 + startDateTime.Second()
	stopSecond := stopDateTime.Hour()*3600 + stopDateTime.Minute()*60 + stopDateTime.Second()

	url := fmt.Sprintf("http://%s:%d/StandardApiAction_getVideoFileInfc.action"+
		"?DevIDNO=%d&LOC=1&CHN=%d&YEAR=%d&MON=%d&DAY=%d&RECTYPE=-1&FILEATTR=2&BEG=%d&END=%d&ARM1=0&ARM2=0&RES=0&STREAM=0&STORE=0&jsession=%s",
		c.gateway, c.apiPort, deviceId, channel, year, month, day, startSecond, stopSecond, c.jsession)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, nil)
	req.Header.Add("user-agent", "vscode-restclient")

	httpClient := http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	if resp.StatusCode != 200 {
		err = fmt.Errorf("fail to send request to server. Status code: %d, info: %+v", resp.StatusCode, cast.ToString(body))
		return
	}

	if err = json.Unmarshal(body, &result); err != nil {
		return
	}

	return
}

type DownloadTaskResponse struct {
	Result     uint `json:"result"`
	OldTaskAll struct {
		Dph string `json:"dph"`
		Len uint   `json:"len"`
	} `json:"oldTaskAll"`
}

func (c *Client) DownloadTask(ctx context.Context, url string) (result DownloadTaskResponse, err error) {
	if err = c.checkSession(ctx); err != nil {
		return
	}

	basePath := url[:66]
	paramsDecode, err := urlLib.ParseQuery(url[66:])
	if err != nil {
		return
	}

	params := urlLib.Values{}
	params.Add("jsession", paramsDecode["jsession"][0])
	params.Add("did", paramsDecode["did"][0])
	params.Add("fbtm", paramsDecode["fbtm"][0])
	params.Add("fetm", paramsDecode["fetm"][0])
	params.Add("sbtm", paramsDecode["sbtm"][0])
	params.Add("setm", paramsDecode["setm"][0])
	params.Add("fph", paramsDecode["fph"][0])
	params.Add("vtp", paramsDecode["vtp"][0])
	params.Add("len", paramsDecode["len"][0])
	params.Add("chn", paramsDecode["chn"][0])
	params.Add("dtp", paramsDecode["dtp"][0])
	paramsEncode := params.Encode()

	urlCorrect := fmt.Sprintf("%s%s", basePath, paramsEncode)

	req, _ := http.NewRequestWithContext(ctx, "GET", urlCorrect, nil)

	httpClient := http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	if resp.StatusCode != 200 {
		err = fmt.Errorf("fail to send request to server. Status code: %d, info: %+v", resp.StatusCode, cast.ToString(body))
		return
	}

	if err = json.Unmarshal(body, &result); err != nil {
		return
	}

	return
}

func (c *Client) Download(ctx context.Context, deviceId, length uint, serverPath string) (fileName string, err error) {
	if err = c.checkSession(ctx); err != nil {
		return
	}

	serverPathPars := strings.Split(serverPath, "/")
	fileName = serverPathPars[len(serverPathPars)-1]

	params := urlLib.Values{}
	params.Add("jsession", c.jsession)
	params.Add("DevIDNO", cast.ToString(deviceId))
	params.Add("FILELOC", cast.ToString(1))
	params.Add("FLENGTH", cast.ToString(length))
	params.Add("FOFFSET", cast.ToString(0))
	params.Add("MTYPE", cast.ToString(1))
	params.Add("FPATH", serverPath)
	params.Add("SAVENAME", fileName)
	paramsEncode := params.Encode()

	url := fmt.Sprintf("http://%s:%d/3/5?DownType=3&%s", c.gateway, c.downloadPort, paramsEncode)

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)

	httpClient := http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	tmpfile, err := os.Create(fileName)
	if err != nil {
		return
	}
	defer tmpfile.Close()

	tmpfile.Write(body)

	return
}

func (c *Client) VideoDownloadHandler(ctx context.Context, deviceId, channel uint, startDateTime, stopDateTime time.Time) (err error) {
	startDateTimeCycle := startDateTime
	stopDateTimeCycle := stopDateTime

	for {

		if startDateTimeCycle.After(stopDateTime) {
			break
		}

		stopDateTimeCycle = startDateTimeCycle.Add(5 * time.Second)

		videoInfo, err := c.GetVideoFileInfo(ctx, startDateTimeCycle, stopDateTimeCycle, deviceId, channel)
		if err != nil {
			return err
		}

		waitDownloadToServer := func(ctx context.Context, startDelaySec uint, url string,
		) (downloadTask DownloadTaskResponse, err error) {
			for {
				select {
				case <-ctx.Done():
					err = ctx.Err()
					return
				default:
					time.Sleep(cast.ToDuration(startDelaySec) * time.Second)

					downloadTask, err = c.DownloadTask(ctx, url)
					if err != nil {
						return
					}

					switch downloadTask.Result {
					case 0:
						startDelaySec *= 2
						continue
					case 11:
						return
					default:
						startDelaySec *= 2
						continue
					}
				}
			}
		}

		for _, val := range videoInfo.Files {
			downloadTask, err := waitDownloadToServer(ctx, 180, val.DownTaskUrl)
			if err != nil {
				return err
			}

			fileName, err := c.Download(ctx, deviceId, downloadTask.OldTaskAll.Len, downloadTask.OldTaskAll.Dph)
			if err != nil {
				return err
			}

			fileNameParse := strings.Split(fileName, ".")
			cmd := exec.Command("ffmpeg", "-i", fileName, fileNameParse[0]+".mp4")

			out, err := cmd.Output()
			if err != nil {
				fmt.Println(err)
			}
			fmt.Println(out)
		}
		startDateTimeCycle = startDateTimeCycle.Add(5 * time.Second)
	}
	return
}

type GetTrackDetailResponse struct {
	Result     uint `json:"result"`
	Pagination struct {
		CurrentPage uint `json:"currentPage"`
		TotalPages  uint `json:"totalPages"`
		PageRecords uint `json:"pageRecords"`
	} `json:"pagination"`
	Tracks []struct {
		Id              string `json:"id"`
		Lng             uint   `json:"lng"`
		Lat             uint   `json:"lat"`
		Speed           uint   `json:"sp"`
		StatusRegister1 uint32 `json:"s1"`
		Time            string `json:"gt"`
	} `json:"tracks"`
}

func (c *Client) GetTrackDetail(ctx context.Context, deviceId, page, pagination uint, startDateTime, stopDateTime time.Time,
) (result GetTrackDetailResponse, err error) {
	if err = c.checkSession(ctx); err != nil {
		return
	}

	params := urlLib.Values{}
	params.Add("jsession", c.jsession)
	params.Add("devIdno", cast.ToString(deviceId))
	params.Add("begintime", cast.ToString(startDateTime)[:19])
	params.Add("endtime", cast.ToString(stopDateTime)[:19])
	params.Add("currentPage", cast.ToString(page))
	params.Add("pageRecords", cast.ToString(pagination))
	paramsEncode := params.Encode()

	url := fmt.Sprintf("http://%s:%d/StandardApiAction_queryTrackDetail.action?%s",
		c.gateway, c.apiPort, paramsEncode)

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)

	httpClient := http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	if resp.StatusCode != 200 {
		err = fmt.Errorf("fail to send request to server. Status code: %d, info: %+v", resp.StatusCode, cast.ToString(body))
		return
	}

	if err = json.Unmarshal(body, &result); err != nil {
		return
	}

	return
}
