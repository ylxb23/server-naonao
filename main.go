package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

var (
	// 存储文件路径
	userHome, _ = os.UserHomeDir()
	dataPath    = userHome + "/data/naonao/data/"
	uploadPath  = userHome + "/data/naonao/images/"
	requestPath = "http://127.0.0.1:8080/image/"
	// 微信配置信息
	wxAppId     = "wx3302905cf62be66c"
	wxAppSecret = "d901d71313cc231311a0b5794bc483a9"
	wxGrantType = "authorization_code"
)

func main() {
	// 创建 gin 服务
	r := gin.Default()
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Hello NaoNao",
		})
	})
	// 判断 uploadPath目录是否存在，不存在则创建该文件夹
	if _, err := os.Stat(uploadPath); os.IsNotExist(err) {
		os.MkdirAll(uploadPath, os.ModePerm)
	}
	// 接收上传的文件
	r.POST("/upload", uploadAndSaveFile)
	r.GET("/image/:filename", getFile)
	// 保存卡片信息
	r.POST("/card", saveCardInfo)
	r.GET("/cards/:openid", getCardListInfo)
	// request for: /wx/user?js_code=xxxxxx
	r.GET("/wx/user", getWxUserInfoByJsCode)
	r.Run(":8080") // 监听端口"
}

func getCardListInfo(c *gin.Context) {
	openid := c.Param("openid")
	if openid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Openid is required"})
		return
	}
	dataUri := dataPath + openid + ".json"
	if _, err := os.Stat(dataUri); os.IsNotExist(err) {
		// 为空
		fmt.Printf("用户卡片数据为空, openid[%s], err: %v\n", openid, err)
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
	c.JSON(http.StatusOK, gin.H{"cards": cards})
}

func saveCardInfo(c *gin.Context) {
	// 请求参数是json格式的 CardItemRequest
	var cardItem CardItem
	if err := c.ShouldBindJSON(&cardItem); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

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
	CardItem  CardItem `json:"card"`
	Openid    string   `json:"openid"`
	Operation string   `json:"operation"` // 添加、删除、更新
}

type CardItem struct {
	Sort       int    `json:"sort"`
	Type       string `json:"type"`
	Title      string `json:"title"`
	Date       string `json:"date"`
	Background string `json:"background"`
}

type WxUser struct {
	Errcode    int    `json:"errcode"`
	Errmsg     string `json:"errmsg"`
	SessionKey string `json:"session_key"`
	OpenId     string `json:"openid"`
}

func getFile(c *gin.Context) {
	filename := c.Param("filename")
	path := uploadPath + filename
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
	savePath := uploadPath + md5Hash + fileSuffix
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
