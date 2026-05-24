package logmodel

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ProxyLog는 proxy_logs MongoDB 컬렉션의 도큐먼트다.
// Java의 ProxyReqResLog에 대응한다.
type ProxyLog struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"  json:"id"`
	TraceID   string             `bson:"traceId"        json:"traceId"`
	Timestamp time.Time          `bson:"timestamp"      json:"timestamp"`
	Timezone  string             `bson:"timezone"       json:"timezone"`
	Direction string             `bson:"direction"      json:"direction"`
	Request   *RequestLog        `bson:"request"        json:"request"`
	Response  *ResponseLog       `bson:"response"       json:"response"`
	LatencyMs int64              `bson:"latencyMs"      json:"latencyMs"`
	TokenID   int64              `bson:"tokenId"        json:"tokenId"`
	UserID    int64              `bson:"userId"         json:"userId"`
	Origin    *OriginLog         `bson:"origin"         json:"origin"`
	Result    string             `bson:"result"         json:"result"`
	Error     *ErrorLog          `bson:"error,omitempty" json:"error,omitempty"`
}

type RequestLog struct {
	Method        string            `bson:"method"        json:"method"`
	Scheme        string            `bson:"scheme"        json:"scheme"`
	Host          string            `bson:"host"          json:"host"`
	Path          string            `bson:"path"          json:"path"`
	Query         string            `bson:"query"         json:"query"`
	Headers       map[string]string `bson:"headers"       json:"headers"`
	IP            string            `bson:"ip"            json:"ip"`
	UserAgent     string            `bson:"userAgent"     json:"userAgent"`
	Body          string            `bson:"body"          json:"body"`
	BodyTruncated bool              `bson:"bodyTruncated" json:"bodyTruncated"`
	ContentLength int64             `bson:"contentLength" json:"contentLength"`
}

type ResponseLog struct {
	Status        int               `bson:"status"        json:"status"`
	Headers       map[string]string `bson:"headers"       json:"headers"`
	Body          string            `bson:"body"          json:"body"`
	BodyTruncated bool              `bson:"bodyTruncated" json:"bodyTruncated"`
	ContentLength int64             `bson:"contentLength" json:"contentLength"`
}

type OriginLog struct {
	Referer      string `bson:"referer"      json:"referer"`
	ForwardedFor string `bson:"forwardedFor" json:"forwardedFor"`
	Geo          string `bson:"geo"          json:"geo"`
}

type ErrorLog struct {
	Type    string `bson:"type"    json:"type"`
	Message string `bson:"message" json:"message"`
	Code    int    `bson:"code"    json:"code"`
	Stack   string `bson:"stack"   json:"stack"`
}

const (
	ResultSuccess = "SUCCESS"
	ResultError   = "ERROR"

	DirectionBrowserToServer = "BROWSER_TO_SERVER"
	DirectionServerToServer  = "SERVER_TO_SERVER"
)
