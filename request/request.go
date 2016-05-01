package request

import (
	"bytes"
	"crypto/md5"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/STNS/STNS/stns"
	"github.com/STNS/libnss_stns/config"
	"github.com/STNS/libnss_stns/logger"
	"github.com/STNS/libnss_stns/settings"
)

type Request struct {
	ApiPath string
	Config  *config.Config
}

func NewRequest(config *config.Config, paths ...string) (*Request, error) {
	logger.Setlog()
	r := Request{}

	r.Config = config
	r.ApiPath = strings.Join(paths, "/")

	return &r, nil
}

func (r *Request) GetRaw() ([]byte, error) {
	var lastError error
	rand.Seed(time.Now().UnixNano())
	perm := rand.Perm(len(r.Config.ApiEndPoint))

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: !r.Config.SslVerify}
	http.DefaultTransport.(*http.Transport).Dial = (&net.Dialer{
		Timeout:   settings.HTTP_TIMEOUT * time.Second,
		KeepAlive: 30 * time.Second,
	}).Dial

	for _, v := range perm {
		endPoint := r.Config.ApiEndPoint[v]
		url := strings.TrimRight(endPoint, "/") + "/" + strings.TrimLeft(path.Clean(r.ApiPath), "/")
		req, err := http.NewRequest("GET", url, nil)

		if err != nil {
			lastError = err
			continue
		}

		if r.Config.User != "" && r.Config.Password != "" {
			req.SetBasicAuth(r.Config.User, r.Config.Password)
		}

		if r.checkLockFile(endPoint) {
			res, err := http.DefaultClient.Do(req)

			if err != nil {
				r.writeLockFile(endPoint)
				lastError = err
				continue
			}

			defer res.Body.Close()
			body, err := ioutil.ReadAll(res.Body)

			if err != nil {
				lastError = err
				continue
			}

			if res.StatusCode == http.StatusOK {
				return body, nil
			}

		}
	}
	return nil, lastError
}

func (r *Request) checkLockFile(endPoint string) bool {
	fileName := "/tmp/libnss_stns." + r.GetMD5Hash(endPoint)
	_, err := os.Stat(fileName)

	// lockfile not exists
	if err != nil {
		return true
	}

	buff, err := ioutil.ReadFile(fileName)
	if err != nil {
		log.Println(err)
		os.Remove(fileName)
		return false
	}

	buf := bytes.NewBuffer(buff)
	lastTime, err := binary.ReadVarint(buf)
	if err != nil {
		log.Println(err)
		os.Remove(fileName)
		return false
	}

	if time.Now().Unix() > lastTime+settings.LOCK_TIME || lastTime > time.Now().Unix()+settings.LOCK_TIME {
		os.Remove(fileName)
		return true
	}

	return false
}

func (r *Request) writeLockFile(endPoint string) {
	fileName := "/tmp/libnss_stns." + r.GetMD5Hash(endPoint)
	now := time.Now()
	log.Println("create lockfile:" + endPoint + " time:" + now.String() + " unix_time:" + strconv.FormatInt(now.Unix(), 10))

	result := make([]byte, binary.MaxVarintLen64)
	binary.PutVarint(result, now.Unix())
	ioutil.WriteFile(fileName, result, os.ModePerm)
}

func (r *Request) Get() (stns.Attributes, error) {
	var attr stns.Attributes

	body, err := r.GetRaw()

	if err != nil {
		return nil, err
	}

	if len(body) > 0 {
		err = json.Unmarshal(body, &attr)

		if err != nil {
			return nil, err
		}
	}

	return attr, nil
}

func (r *Request) GetMD5Hash(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}
