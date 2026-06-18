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

// 东方财富返回结构
type EastMoneyResponse struct {
	Rc   int    `json:"rc"`
	Rt   int    `json:"rt"`
	Svr  int    `json:"svr"`
	Lt   int    `json:"lt"`
	Full int    `json:"full"`
	Data struct {
		F43 float64 `json:"f43"`
	} `json:"data"`
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

		time.Sleep(500 * time.Millisecond)
	}
}

func checkStock(code, name string, targetPrice float64) {

	var emCode string

	switch {
	case strings.HasPrefix(code, "sh"):
		emCode = "1." + strings.TrimPrefix(code, "sh")

	case strings.HasPrefix(code, "sz"):
		emCode = "0." + strings.TrimPrefix(code, "sz")

	default:
		emCode = code
	}

	url := fmt.Sprintf(
		"https://push2.eastmoney.com/api/qt/stock/get?secid=%s&fields=f43",
		emCode,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("❌ [%s] 创建请求失败: %v\n", name, err)
		return
	}

	req.Header.Set(
		"User-Agent",
		"Mozilla/5.0",
	)

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

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
		fmt.Printf("❌ [%s] 读取响应失败\n", name)
		return
	}

	var result EastMoneyResponse

	err = json.Unmarshal(body, &result)

	if err != nil {
		fmt.Printf(
			"❌ [%s] JSON解析失败: %v\n",
			name,
			err,
		)
		return
	}

	currentPrice := result.Data.F43 / 100

	if currentPrice <= 0 {
		fmt.Printf(
			"❌ [%s] 获取价格失败，返回值=%.2f\n",
			name,
			currentPrice,
		)
		return
	}

	fmt.Printf(
		"📊 [%s] 当前价 %.2f 元 | 目标价 %.2f 元\n",
		name,
		currentPrice,
		targetPrice,
	)

	if currentPrice <= targetPrice {

		fmt.Printf(
			"🔥 [%s] 已达到目标价\n",
			name,
		)

		sendToWeChat(
			name,
			code,
			currentPrice,
			targetPrice,
		)
	}
}

func sendToWeChat(
	name string,
	code string,
	current float64,
	target float64,
) {

	webhookURL := os.Getenv("WECOM_WEBHOOK")

	if webhookURL == "" {
		fmt.Println("⚠️ 未配置企业微信Webhook")
		return
	}

	msgContent := fmt.Sprintf(
		"### 📈 股票价格提醒\n"+
			"> 股票名称：`%s` (%s)\n"+
			"> 当前价格：<font color=\"warning\">%.2f 元</font>\n"+
			"> 目标价格：%.2f 元\n\n"+
			"> 💡 已达到预设买入区间",
		name,
		code,
		current,
		target,
	)

	payload := WeChatMsg{
		MsgType: "markdown",
		Markdown: WeChatMarkdown{
			Content: msgContent,
		},
	}

	jsonData, _ := json.Marshal(payload)

	resp, err := http.Post(
		webhookURL,
		"application/json",
		bytes.NewBuffer(jsonData),
	)

	if err != nil {
		fmt.Printf(
			"❌ [%s] 企业微信发送失败: %v\n",
			name,
			err,
		)
		return
	}

	defer resp.Body.Close()

	fmt.Printf(
		"🎉 [%s] 企业微信通知成功\n",
		name,
	)
}
