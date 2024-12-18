package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/gin-gonic/gin"
)

var (
	// 启动端口
	port = ":8080"
	// 存储文件路径
	userHome, _ = os.UserHomeDir()
	basePath    = userHome + "/data/naonao/"
	dataPath    = basePath + "data/"
	imgsPath    = basePath + "images/"
	requestPath = fmt.Sprintf("http://%s%s/image/", localIp(), port)
	// 微信配置信息
	wxAppId     = "wx3302905cf62be66c"
	wxAppSecret = "d901d71313cc231311a0b5794bc483a9"
	wxGrantType = "authorization_code"
)

func main() {
	// 创建 gin 服务
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Hello NaoNao",
		})
	})
	// 判断 dataPath,imgsPath 目录是否存在，不存在则创建该文件夹
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		os.MkdirAll(dataPath, os.ModePerm)
	}
	if _, err := os.Stat(imgsPath); os.IsNotExist(err) {
		os.MkdirAll(imgsPath, os.ModePerm)
	}
	// 接收上传的文件
	r.POST("/upload", uploadAndSaveFile)
	r.GET("/image/:filename", getFile)
	// 保存卡片信息
	r.POST("/card", saveCardInfo)
	r.GET("/cards/:openid", getCardListInfo)
	// request for: /wx/user?js_code=xxxxxx
	r.GET("/wx/user", getWxUserInfoByJsCode)
	fmt.Println("http://192.168.43.167:8080")
	r.Run(port) // 监听端口"
	fmt.Println("服务启动成功, 端口: ", port)
}

// 获取当前本地ip
func localIp() string {
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		// for dev
		return "192.168.43.167"
	}
	for _, address := range addrs {
		// 检查ip地址判断是否回环地址
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "192.168.43.167"
}

func dataLocalUri(openid string) string {
	return fmt.Sprintf("%s%s.json", dataPath, openid)
}

/**
 * 根据用户openid获取用户的卡片列表
 */
func getCardListInfo(c *gin.Context) {
	openid := c.Param("openid")
	if openid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Openid is required"})
		return
	}
	dataUri := dataLocalUri(openid)
	if _, err := os.Stat(dataUri); os.IsNotExist(err) {
		// 为空
		fmt.Printf("用户卡片数据为空, openid[%s], info: %v\n", openid, err)
		c.JSON(http.StatusOK, gin.H{"cards": []string{}})
		return
	}
	file, err := os.Open(dataUri)
	if err != nil {
		fmt.Printf("卡片列表数据文件打开异常, openid[%s], err: %v\n", openid, err)
		c.JSON(http.StatusOK, gin.H{"cards": []string{}})
		return
	}
	defer file.Close()
	var cards []CardItem
	if err := json.NewDecoder(file).Decode(&cards); err != nil {
		fmt.Printf("反序列化卡片列表异常, openid[%s], err: %v\n", openid, err)
		c.JSON(http.StatusOK, gin.H{"cards": []string{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"cards": cards, "size": len(cards)})
}

func saveCardInfo(c *gin.Context) {
	// 请求参数是json格式的 CardItemRequest
	var cardItemRequest CardItemRequest
	if err := c.ShouldBindJSON(&cardItemRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 检查卡片信息是否合法
	if ok, msg := cardContentCheck(cardItemRequest); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	var cards []CardItem
	// 读取所有这个opendid
	dataUri := dataLocalUri(cardItemRequest.Openid)
	_, err := os.Stat(dataUri)
	if err != nil {
		fmt.Printf("用户卡片列表还是空的,将新建初始化, openid[%s], info: %v\n", cardItemRequest.Openid, err)
	} else {
		file, err := os.Open(dataUri)
		if err != nil {
			fmt.Printf("卡片列表数据文件打开异常, openid[%s], err: %v\n", cardItemRequest.Openid, err)
			c.JSON(http.StatusOK, gin.H{"cards": []string{}})
			return
		}
		defer file.Close()
		if err := json.NewDecoder(file).Decode(&cards); err != nil {
			fmt.Printf("反序列化卡片列表异常, openid[%s], err: %v\n", cardItemRequest.Openid, err)
			c.JSON(http.StatusOK, gin.H{"cards": []string{}})
			return
		}
		fmt.Printf("读取卡片列表数据成功, openid[%s], cards: %v\n", cardItemRequest.Openid, cards)
		// 根据 sort属性排序cards
		sort.Slice(cards, func(i, j int) bool {
			return cards[i].Sort < cards[j].Sort
		})
	}

	if cardItemRequest.Operation == "add" {
		// 判断同类型的卡片，Title 是否存在，如果存在则直接返回成功
		for _, card := range cards {
			if card.Title == cardItemRequest.Card.Title {
				c.JSON(http.StatusOK, gin.H{"message": "卡片已存在", "cards": cards, "total": len(cards)})
				return
			}
		}
		if cardItemRequest.Card.Sort == 0 {
			if len(cards) > 0 {
				cardItemRequest.Card.Sort = cards[len(cards)-1].Sort + 1
			} else {
				cardItemRequest.Card.Sort = 1
			}
		}
		cards = append(cards, cardItemRequest.Card)
		fmt.Printf("添加卡片数据成功, openid[%s], cards: %v\n", cardItemRequest.Openid, cards)
	} else if cardItemRequest.Operation == "delete" {
		// 删除卡片
		for i, card := range cards {
			if card.Title == cardItemRequest.Card.Title {
				// 删除卡片
				cards = append(cards[:i], cards[i+1:]...)
				fmt.Printf("删除卡片数据成功, openid[%s], cards: %v\n", cardItemRequest.Openid, cards)
				break
			}
		}
	} else if cardItemRequest.Operation == "update" {
		// 更新卡片
		for i, card := range cards {
			if card.Title == cardItemRequest.Card.Title {
				// 替换卡片
				cards[i] = cardItemRequest.Card
				fmt.Printf("更新卡片数据成功, openid[%s], cards: %v\n", cardItemRequest.Openid, cards)
				break
			}
		}
	}

	data, err := json.Marshal(cards)
	if err != nil {
		fmt.Printf("序列化卡片列表异常, openid[%s], err: %v\n", cardItemRequest.Openid, err)
		c.JSON(http.StatusOK, gin.H{"cards": []string{}})
		return
	}
	os.WriteFile(dataUri, data, 0644)
	fmt.Printf("写入卡片列表数据成功, openid[%s], cards: %v\n", cardItemRequest.Openid, cards)
	c.JSON(http.StatusOK, gin.H{"message": "卡片数据保存成功", "cards": cards, "total": len(cards)})
}

/**
 * 根据微信的 jsCode 获取用户信息
 */
func getWxUserInfoByJsCode(c *gin.Context) {
	// 获取GET的请求参数 js_code
	jsCode := c.Query("js_code")
	fmt.Printf("请求的JsCode: %s", jsCode)
	wxUserUri := fmt.Sprintf("https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=%s", wxAppId, wxAppSecret, jsCode, wxGrantType)
	fmt.Printf("请求的微信接口: %s \n", wxUserUri)
	resp, err := http.Get(wxUserUri)
	if err != nil {
		fmt.Printf("微信用户信息接口请求失败: %v \n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get wx user info"})
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		var buf bytes.Buffer
		_, err := io.Copy(&buf, resp.Body)
		if err != nil {
			fmt.Printf("微信用户信息接口请求失败: %v \n", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get wx user info"})
		}
		fmt.Printf("请求微信用户信息接口成功: %s \n", buf.String())
		// buf.String() 转换成 WxUser对象
		var wxUser WxUser
		if err := json.Unmarshal(buf.Bytes(), &wxUser); err != nil {
			fmt.Printf("微信用户信息接口请求失败: %v \n", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get wx user info"})
		}
		// 获取微信用户信息成功
		c.JSON(http.StatusOK, wxUser)
	}
}

type CardItemRequest struct {
	Card      CardItem `json:"card"`
	Openid    string   `json:"openid"`
	Operation string   `json:"operation"` // 添加、删除、更新
}

type CardItem struct {
	Sort       int             `json:"sort"`
	Type       int             `json:"type"` // 0-Empty, 2-Anniversary, 3-Countdown, 4-CountdownList, 5-Progress, 6-ToDoList
	Title      string          `json:"title"`
	Date       string          `json:"date"`
	Background string          `json:"background"`
	List       []NamedDateItem `json:"list"`
}

type NamedDateItem struct {
	Name   string `json:"name"`
	Date   string `json:"date"`
	Avatar string `json:"avatar"`
}

type WxUser struct {
	Errcode    int    `json:"errcode"`
	Errmsg     string `json:"errmsg"`
	SessionKey string `json:"session_key"`
	OpenId     string `json:"openid"`
}

func getFile(c *gin.Context) {
	filename := c.Param("filename")
	path := imgsPath + filename
	// 判断文件路径是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}
	c.File(path)
}

func uploadAndSaveFile(c *gin.Context) {
	// 创建一个多部分表单读取器
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse multipart form"})
		return
	}

	// 假设只有一个文件上传
	files := form.File["file"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	file := files[0]
	filename := file.Filename
	// 获取文件后缀名(带 .)
	fileSuffix := filepath.Ext(filename)
	// 打开上传的文件
	fileReader, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open uploaded file"})
		return
	}
	defer fileReader.Close()

	// 读取文件内容到byte数组
	var buffer bytes.Buffer
	if _, err := io.Copy(&buffer, fileReader); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file content"})
		return
	}

	// 计算文件内容的MD5哈希值
	hasher := md5.New()
	if _, err := hasher.Write(buffer.Bytes()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to compute MD5 hash"})
		return
	}
	md5Hash := hex.EncodeToString(hasher.Sum(nil))

	// 构建保存的文件路径
	savePath := imgsPath + md5Hash + fileSuffix
	fmt.Printf("上传的文件: %s, 保存到: %s \n", filename, savePath)
	// 保存文件到指定路径
	out, err := os.Create(savePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create file on server"})
		return
	}
	defer out.Close()

	if _, err = io.Copy(out, &buffer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write file to disk"})
		return
	}

	// 返回保存的文件路径
	c.JSON(http.StatusOK, gin.H{"success": requestPath + md5Hash + fileSuffix})
}
