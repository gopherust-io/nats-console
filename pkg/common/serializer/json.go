package serializer

import (
	"github.com/bytedance/sonic"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/pkg/common/jsonkeys"
)

const (
	jsonContentType = "application/json"
)

func WriteJSON(ctx *fasthttp.RequestCtx, status int, v any) {
	data, err := sonic.Marshal(v)
	if err != nil {
		WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	data, err = jsonkeys.ToCamelCaseJSON(data)
	if err != nil {
		WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	ctx.SetStatusCode(status)
	ctx.SetContentType(jsonContentType)
	ctx.SetBody(data)
}

func WriteRawJSON(ctx *fasthttp.RequestCtx, data []byte) {
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType(jsonContentType)
	ctx.SetBody(data)
}

type errorResponse struct {
	Error string `json:"error"`
}

func WriteError(ctx *fasthttp.RequestCtx, status int, err error) {
	WriteJSON(ctx, status, errorResponse{Error: err.Error()})
}

func UnmarshalRequest(body []byte, v any) error {
	return sonic.Unmarshal(body, v)
}

func UnmarshalNATSRequest(body []byte, v any) error {
	if len(body) == 0 {
		return sonic.Unmarshal(body, v)
	}
	snakeBody, err := jsonkeys.FromCamelCaseJSON(body)
	if err != nil {
		return err
	}
	return sonic.Unmarshal(snakeBody, v)
}
