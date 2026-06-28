package serializer

import (
	"github.com/bytedance/sonic"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/pkg/common/bufpool"
)

const (
	jsonContentType = "application/json"
)

func marshalJSON(v any) ([]byte, error) {
	buf := bufpool.GetBuffer()
	defer bufpool.PutBuffer(buf)
	enc := sonic.ConfigDefault.NewEncoder(buf)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	out := make([]byte, buf.Len())
	copy(out, buf.Bytes())
	return out, nil
}

func WriteJSON(ctx *fasthttp.RequestCtx, status int, v any) {
	data, err := marshalJSON(v)
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
