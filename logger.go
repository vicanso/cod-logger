// Copyright 2018 tree xie
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logger

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/vicanso/elton"
)

const (
	host             = "host"
	method           = "method"
	path             = "path"
	proto            = "proto"
	query            = "query"
	remote           = "remote"
	realIP           = "real-ip"
	scheme           = "scheme"
	uri              = "uri"
	referer          = "referer"
	userAgent        = "userAgent"
	when             = "when"
	whenISO          = "when-iso"
	whenUTCISO       = "when-utc-iso"
	whenUnix         = "when-unix"
	whenISOMs        = "when-iso-ms"
	whenUTCISOMs     = "when-utc-iso-ms"
	size             = "size"
	sizeHuman        = "size-human"
	status           = "status"
	latency          = "latency"
	latencyMs        = "latency-ms"
	cookie           = "cookie"
	payloadSize      = "payload-size"
	payloadSizeHuman = "payload-size-human"
	requestHeader    = "requestHeader"
	responseHeader   = "responseHeader"
	httpProto        = "HTTP"
	httpsProto       = "HTTPS"

	kbytes = 1024
	mbytes = 1024 * 1024

	// CommonFormat common log format
	CommonFormat = "{real-ip} {when-iso} {method} {uri} {status}"
)

type (
	// Tag log tag
	Tag struct {
		category string
		data     string
	}
	// OnLog on log function
	OnLog func(string, *elton.Context)
	// Config logger config
	Config struct {
		Format  string
		OnLog   OnLog
		Skipper elton.Skipper
	}
)

// byteSliceToString converts a []byte to string without a heap allocation.
func byteSliceToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
func cut(str string) string {
	l := len(str)
	if l == 0 {
		return str
	}
	ch := str[l-1]
	if ch == '0' || ch == '.' {
		return cut(str[0 : l-1])
	}
	return str
}

// getHumanReadableSize get the size for human
func getHumanReadableSize(size int) string {
	if size < kbytes {
		return fmt.Sprintf("%dB", size)
	}
	fSize := float64(size)
	if size < mbytes {
		s := cut(fmt.Sprintf("%.2f", (fSize / kbytes)))
		return s + "KB"
	}
	s := cut(fmt.Sprintf("%.2f", (fSize / mbytes)))
	return s + "MB"
}

// getTimeConsuming 获取使用耗时(ms)
func getTimeConsuming(startedAt time.Time) int {
	v := startedAt.UnixNano()
	now := time.Now().UnixNano()
	return int((now - v) / 1000000)
}

// parse 转换日志的输出格式
func parse(desc []byte) []*Tag {
	reg := regexp.MustCompile(`\{[\S]+?\}`)

	index := 0
	arr := make([]*Tag, 0)
	fillCategory := "fill"
	for {
		result := reg.FindIndex(desc[index:])
		if result == nil {
			break
		}
		start := index + result[0]
		end := index + result[1]
		if start != index {
			arr = append(arr, &Tag{
				category: fillCategory,
				data:     byteSliceToString(desc[index:start]),
			})
		}
		k := desc[start+1 : end-1]
		switch k[0] {
		case byte('~'):
			arr = append(arr, &Tag{
				category: cookie,
				data:     byteSliceToString(k[1:]),
			})
		case byte('>'):
			arr = append(arr, &Tag{
				category: requestHeader,
				data:     byteSliceToString(k[1:]),
			})
		case byte('<'):
			arr = append(arr, &Tag{
				category: responseHeader,
				data:     byteSliceToString(k[1:]),
			})
		default:
			arr = append(arr, &Tag{
				category: byteSliceToString(k),
				data:     "",
			})
		}
		index = result[1] + index
	}
	if index < len(desc) {
		arr = append(arr, &Tag{
			category: fillCategory,
			data:     byteSliceToString(desc[index:]),
		})
	}
	return arr
}

// format 格式化访问日志信息
func format(c *elton.Context, tags []*Tag, startedAt time.Time) string {
	fn := func(tag *Tag) string {
		switch tag.category {
		case host:
			return c.Request.Host
		case method:
			return c.Request.Method
		case path:
			p := c.Request.URL.Path
			if p == "" {
				p = "/"
			}
			return p
		case proto:
			return c.Request.Proto
		case query:
			return c.Request.URL.RawQuery
		case remote:
			return c.Request.RemoteAddr
		case realIP:
			return c.RealIP()
		case scheme:
			if c.Request.TLS != nil {
				return httpsProto
			}
			return httpProto
		case uri:
			return c.Request.RequestURI
		case cookie:
			cookie, err := c.Cookie(tag.data)
			if err != nil {
				return ""
			}
			return cookie.Value
		case requestHeader:
			return c.Request.Header.Get(tag.data)
		case responseHeader:
			return c.GetHeader(tag.data)
		case referer:
			return c.Request.Referer()
		case userAgent:
			return c.Request.UserAgent()
		case when:
			return time.Now().Format(time.RFC1123Z)
		case whenISO:
			return time.Now().Format(time.RFC3339)
		case whenUTCISO:
			return time.Now().UTC().Format("2006-01-02T15:04:05Z")
		case whenISOMs:
			return time.Now().Format("2006-01-02T15:04:05.999Z07:00")
		case whenUTCISOMs:
			return time.Now().UTC().Format("2006-01-02T15:04:05.999Z")
		case whenUnix:
			return strconv.FormatInt(time.Now().Unix(), 10)
		case status:
			return strconv.Itoa(c.StatusCode)
		case payloadSize:
			return strconv.Itoa(len(c.RequestBody))
		case payloadSizeHuman:
			return getHumanReadableSize(len(c.RequestBody))
		case size:
			bodyBuf := c.BodyBuffer
			if bodyBuf == nil {
				return "0"
			}
			return strconv.Itoa(bodyBuf.Len())
		case sizeHuman:
			bodyBuf := c.BodyBuffer
			if bodyBuf == nil {
				return "0B"
			}
			return getHumanReadableSize(bodyBuf.Len())
		case latency:
			return time.Since(startedAt).String()
		case latencyMs:
			ms := getTimeConsuming(startedAt)
			return strconv.Itoa(ms)
		default:
			return tag.data
		}
	}

	arr := make([]string, 0, len(tags))
	for _, tag := range tags {
		arr = append(arr, fn(tag))
	}
	return strings.Join(arr, "")
}

// GenerateLog generate log function
func GenerateLog(layout string) func(*elton.Context, time.Time) string {
	tags := parse([]byte(layout))
	return func(c *elton.Context, startedAt time.Time) string {
		return format(c, tags, startedAt)
	}
}

// New create a logger middleware
func New(config Config) elton.Handler {
	if config.Format == "" {
		panic("logger require format")
	}
	if config.OnLog == nil {
		panic("logger require on log function")
	}
	tags := parse([]byte(config.Format))
	skipper := config.Skipper
	if skipper == nil {
		skipper = elton.DefaultSkipper
	}
	return func(c *elton.Context) (err error) {
		if skipper(c) {
			return c.Next()
		}
		startedAt := time.Now()
		err = c.Next()
		str := format(c, tags, startedAt)
		config.OnLog(str, c)
		return err
	}
}
