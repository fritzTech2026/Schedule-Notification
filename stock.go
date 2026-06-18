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
)

// 企业微信消息结构体
type WeChatMarkdown struct {
	Content string `json:"content"`
}
type WeChatMsg struct {
	MsgType  string         `json:"msgtype"`
	Markdown WeChatMarkdown `json:"markdown"`
}

func main() {
	// 1. 配置你要监控的股票和目标价格（也可以通过环境变量传入）
	stockCode := "sh600519"    // 示例：贵州茅台
	stockName := "贵州茅台"
	targetPrice := 1500.0      // 你的目标提醒价格

	// 2. 调用新浪股票 API 获取数据
	url := fmt.Sprintf("http://sinajs.cn", stockCode)
	req, _ := http.NewRequest("GET", url, nil)
	// 新浪接口需要设置 Referer 伪装，否则会报错
	req.Header.Set("Referer", "https://sina.com.cn")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("股票接口请求失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	// 转换编码（新浪返回的是 gbk，如果名称乱码，我们直接用英文或硬编码的中文）
	dataStr := string(body)

	// 3. 解析当前价格
	// 新浪返回格式: var hq_str_sh600519="贵州茅台,开盘价,昨收价,当前价格,...";
	if !strings.Contains(dataStr, "\"") {
		fmt.Println("接口返回数据异常")
		return
	}
	content := strings.Split(dataStr, "\"")[1]
	parts := strings.Split(content, ",")
	if len(parts) < 4 {
		fmt.Println("股票代码可能不正确或无数据")
		return
	}

	currentPrice, _ := strconv.ParseFloat(parts[3], 64) // 第4个元素是当前价格
	fmt.Printf("当前[%s]价格为: %.2f，目标价格为: %.2f\n", stockName, currentPrice, targetPrice)

	// 4. 判断是否达到预期（这里以“跌破或等于目标价”为例，如果是想突破买入可以改成 >=）
	if currentPrice <= targetPrice {
		fmt.Println("🚩 达到目标价格！正在触发企业微信通知...")
		sendToWeChat(stockName, stockCode, currentPrice, targetPrice)
	} else {
		fmt.Println("⏳ 未达到目标价格，今天不发送通知。")
	}
}

// 发送企业微信函数
func sendToWeChat(name, code string, current, target float64) {
	webhookURL := os.Getenv("WECOM_WEBHOOK")
	if webhookURL == "" {
		fmt.Println("错误：未配置企业微信 WECOM_WEBHOOK 变量")
		return
	}

	msgContent := fmt.Sprintf("### 📈 股票价格盯盘提醒\n"+
		"> **股票名称**: `%s` (%s)\n"+
		"> **当前价格**: <font color=\"warning\">%.2f 元</font>\n"+
		"> **设定目标**: %.2f 元\n\n"+
		"> 🔥 **提示**: 该股票已跌入您的心理价位，请留意交易机会！", name, code, current, target)

	payload := WeChatMsg{
		MsgType: "markdown",
		Markdown: WeChatMarkdown{
			Content: msgContent,
		},
	}

	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("企业微信发送失败: %v\n", err)
		return
	}
	defer resp.Body.Close()
	fmt.Println("🎉 成功推送到企业微信！")
}
