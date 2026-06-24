package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type WeChatMarkdown struct {
	Content string `json:"content"`
}

type WeChatMsg struct {
	MsgType  string         `json:"msgtype"`
	Markdown WeChatMarkdown `json:"markdown"`
}

type PushPlusMessage struct {
	Token    string `json:"token"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	Template string `json:"template"`
}

func main() {

	configStr := os.Getenv("STOCK_LIST")

	if configStr == "" {
		fmt.Println("❌ 未配置 STOCK_LIST")
		return
	}

	stocks := strings.Split(configStr, ",")

	for _, stock := range stocks {

		parts := strings.Split(stock, ":")

		if len(parts) != 3 {
			fmt.Printf("⚠️ 配置格式错误: %s\n", stock)
			continue
		}

		code := strings.TrimSpace(parts[0])
		name := strings.TrimSpace(parts[1])

		targetPrice, err := strconv.ParseFloat(
			strings.TrimSpace(parts[2]),
			64,
		)

		if err != nil {
			fmt.Printf("⚠️ [%s] 目标价格格式错误\n", name)
			continue
		}

		checkStock(code, name, targetPrice)

		time.Sleep(time.Second)
	}
}

func checkStock(code, name string, targetPrice float64) {

	url := fmt.Sprintf(
		"https://qt.gtimg.cn/q=%s",
		code,
	)

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("❌ [%s] 创建请求失败: %v\n", name, err)
		return
	}

	req.Header.Set(
		"User-Agent",
		"Mozilla/5.0",
	)

	resp, err := client.Do(req)

	if err != nil {
		fmt.Printf("❌ [%s] 请求失败: %v\n", name, err)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf(
			"❌ [%s] HTTP状态异常: %d\n",
			name,
			resp.StatusCode,
		)
		return
	}

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		fmt.Printf(
			"❌ [%s] 响应读取失败: %v\n",
			name,
			err,
		)
		return
	}

	content := string(body)

	fields := strings.Split(content, "~")

	if len(fields) < 4 {

		fmt.Printf(
			"❌ [%s] 腾讯返回格式异常: %s\n",
			name,
			content,
		)

		return
	}

	currentPrice, err := strconv.ParseFloat(
		fields[3],
		64,
	)

	if err != nil {

		fmt.Printf(
			"❌ [%s] 当前价格解析失败: %v\n",
			name,
			err,
		)

		return
	}

	fmt.Printf(
		"📊 [%s] 当前价 %.2f 元 | 目标价 %.2f 元\n",
		name,
		currentPrice,
		targetPrice,
	)



	if targetPrice<0{
		if currentPrice <= -targetPrice {
			fmt.Printf(
				"🔥 [%s] 已下跌到目标价\n",
				name,
			)
			sendToPushPlus(
				name,
				code,
				currentPrice,
				targetPrice,
			)
		}
	}
	if targetPrice>0{
		if currentPrice >= targetPrice {
			sendToPushPlus(
				name,
				code,
				currentPrice,
				targetPrice,
			)
		}
	}

	
}

// func sendToWeChat(
// 	name string,
// 	code string,
// 	current float64,
// 	target float64,
// ) {

// 	webhookURL := os.Getenv("WECOM_WEBHOOK")

// 	fmt.Printf("Webhook长度=%d\n", len(webhookURL))

// 	if len(webhookURL) > 50 {
// 		fmt.Printf(
// 			"Webhook前30=%s\nWebhook后20=%s\n",
// 			webhookURL[:30],
// 			webhookURL[len(webhookURL)-20:],
// 		)
// 	}

// 	if webhookURL == "" {
// 		fmt.Println("⚠️ 未配置 WECOM_WEBHOOK")
// 		return
// 	}

// 	msgContent := fmt.Sprintf(
// 		"### 股票价格提醒\n"+
// 			"> 股票名称：`%s`\n"+
// 			"> 股票代码：`%s`\n"+
// 			"> 当前价格：<font color=\"warning\">%.2f 元</font>\n"+
// 			"> 目标价格：%.2f 元\n\n"+
// 			"> 已达到预设买入区间。",
// 		name,
// 		code,
// 		current,
// 		target,
// 	)

// 	payload := WeChatMsg{
// 		MsgType: "markdown",
// 		Markdown: WeChatMarkdown{
// 			Content: msgContent,
// 		},
// 	}

// 	jsonData, err := json.Marshal(payload)

// 	if err != nil {
// 		fmt.Printf("❌ JSON编码失败: %v\n", err)
// 		return
// 	}

// 	resp, err := http.Post(
// 		webhookURL,
// 		"application/json",
// 		bytes.NewBuffer(jsonData),
// 	)

// 	if err != nil {
// 		fmt.Printf(
// 			"❌ [%s] 企业微信发送失败: %v\n",
// 			name,
// 			err,
// 		)
// 		return
// 	}

// 	defer resp.Body.Close()

// 	respBody, _ := io.ReadAll(resp.Body)

// 	fmt.Printf(
// 		"📨 [%s] 企业微信返回: %s\n",
// 		name,
// 		string(respBody),
// 	)
// }

func sendToPushPlus(
	name string,
	code string,
	current float64,
	target float64,
) {

	token := os.Getenv("PUSHPLUS_TOKEN")

	if token == "" {
		fmt.Println("❌ 未配置 PUSHPLUS_TOKEN")
		return
	}

	content := fmt.Sprintf(
		"%s\n"+
			"代码：%s\n"+
			"当前价格：%.2f 元\n"+
			"目标价格：%.2f 元\n\n",
		name,
		code,
		current,
		target,
	)

	msg := PushPlusMessage{
		Token:    token,
		Title:    "github股票价格提醒",
		Content:  content,
		Template: "txt",
	}

	jsonData, err := json.Marshal(msg)

	if err != nil {
		fmt.Printf("❌ JSON编码失败: %v\n", err)
		return
	}

	resp, err := http.Post(
		"https://www.pushplus.plus/send",
		"application/json",
		bytes.NewBuffer(jsonData),
	)

	if err != nil {
		fmt.Printf("❌ PushPlus发送失败: %v\n", err)
		return
	}

	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	fmt.Printf(
		"📨 PushPlus返回: %s\n",
		string(body),
	)
}
