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

type WeChatMarkdown struct { Content string `json:"content"` }
type WeChatMsg struct { MsgType string `json:"msgtype"`; Markdown WeChatMarkdown `json:"markdown"` }

// 东方财富接口返回的 JSON 结构体
type EastMoneyResponse struct {
	Data struct {
		F43 float64 `json:"f43"` // f43 在东财接口中代表最新现价（单位通常为分，需除以100）
	} `json:"data"`
}

func main() {
	configStr := os.Getenv("STOCK_LIST")
	if configStr == "" {
		fmt.Println("❌ 错误：未配置 STOCK_LIST 变量")
		return
	}

	stocks := strings.Split(configStr, ",")
	for _, stock := range stocks {
		parts := strings.Split(stock, ":")
		if len(parts) != 3 {
			fmt.Printf("⚠️ 忽略错误格式: %s\n", stock)
			continue
		}
		
		code := strings.TrimSpace(parts[0])
		name := strings.TrimSpace(parts[1])
		targetPrice, _ := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)

		checkStock(code, name, targetPrice)
	}
}

func checkStock(code, name string, targetPrice float64) {
	// 转换股票代码格式适应东方财富接口：sh600519 -> 1.600519；sz000002 -> 0.000002
	emCode := ""
	if strings.HasPrefix(code, "sh") {
		emCode = "1." + strings.TrimPrefix(code, "sh")
	} else if strings.HasPrefix(code, "sz") {
		emCode = "0." + strings.TrimPrefix(code, "sz")
	} else {
		emCode = code // 如果你直接在 yml 填了 1.600519 也能兼容
	}

	// 东方财富标准高可用 API 接口
	url := fmt.Sprintf("https://eastmoney.com", emCode)
	
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("❌ [%s] 海外网络请求失败: %v\n", name, err)
		return 
	}
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("❌ [%s] 东方财富接口响应错误，状态码: %d\n", name, resp.StatusCode)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	// 解析标准的 JSON 数据
	var result EastMoneyResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		fmt.Printf("❌ [%s] JSON 解析失败: %v\n", name, err)
		return
	}

	// 东方财富返回的价格数据放大了 100 倍（例如 1500元会返回 150000），需要除以 100 恢复原价
	currentPrice := result.Data.F43 / 100
	if currentPrice <= 0 {
		fmt.Printf("❌ [%s] 获取到了无效价格（可能是非交易时间或股票代码不正确）\n", name)
		return
	}

	fmt.Printf("📊 东财数据 -> [%s] 当前价: %.2f | 目标设定: %.2f\n", name, currentPrice, targetPrice)

	// 判断是否触及目标价
	if currentPrice <= targetPrice {
		sendToWeChat(name, code, currentPrice, targetPrice)
	}
}

func sendToWeChat(name, code string, current, target float64) {
	webhookURL := os.Getenv("WECOM_WEBHOOK")
	if webhookURL == "" { return }

	msgContent := fmt.Sprintf("### 📈 东方财富·抄底提醒\n"+
		"> **股票名称**: `%s` (%s)\n"+
		"> **当前价格**: <font color=\"warning\">%.2f 元</font>\n"+
		"> **心理价位**: %.2f 元\n\n"+
		"> 💡 **提示**: 海外节点监测：价格已跌破预期，可以考虑建仓！", name, code, current, target)

	payload := WeChatMsg{MsgType: "markdown", Markdown: WeChatMarkdown{Content: msgContent}}
	jsonData, _ := json.Marshal(payload)
	_, _ = http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
	fmt.Printf("🎉 [%s] 已成功推送企业微信！\n", name)
}
