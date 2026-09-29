// wsclient là công cụ thử WebSocket cho test_chat_api.sh (thay bản C# Program.cs):
//
//	wsclient <access_token> [giây=5]
//
// Kết nối ws://localhost:5000/ws/chat?access_token=..., in "[ws] connected",
// in mỗi tin nhận được dạng "[ws] <json>" cho tới khi hết giờ, rồi in "[ws] done".
// Không kết nối được → in "[ws] connect-failed: <lý do>" và thoát.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/coder/websocket"
)

func main() {
	token := ""
	if len(os.Args) > 1 {
		token = os.Args[1]
	}
	seconds := 5
	if len(os.Args) > 2 {
		if n, err := strconv.Atoi(os.Args[2]); err == nil {
			seconds = n
		}
	}

	uri := "ws://localhost:5000/ws/chat?access_token=" + url.QueryEscape(token)
	dialCtx, cancelDial := context.WithTimeout(context.Background(), 5*time.Second)
	conn, _, err := websocket.Dial(dialCtx, uri, nil)
	cancelDial()
	if err != nil {
		fmt.Printf("[ws] connect-failed: %v\n", err)
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(-1)
	fmt.Println("[ws] connected")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
	defer cancel()
	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			if !errors.Is(err, context.DeadlineExceeded) && websocket.CloseStatus(err) == -1 {
				fmt.Printf("[ws] socket-error: %v\n", err)
			}
			break
		}
		if typ == websocket.MessageText {
			fmt.Println("[ws] " + string(data))
		}
	}
	fmt.Println("[ws] done")
}
