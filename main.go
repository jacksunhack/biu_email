package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql" // This import seems unused now? Keep for now.
)

// 嵌入 index.html 内容
const indexHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Nyaa~crypted Kitty Note</title>
  <!-- 添加 viewport 元标签 -->
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <!-- 引入 Font Awesome 图标库 -->
  <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0-beta3/css/all.min.css" 
        integrity="sha384-Fo3rlrQkzQk58+ae5ujg3X8bW5g1d28cZbfD3VJjE1KE6L5Q6vhgkGnj4U6JNvQv" crossorigin="anonymous">
  <!-- 引入 Animate.css 动画库 -->
  <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/animate.css/4.1.1/animate.min.css"/>
  <!-- 引入可爱字体 Pangolin -->
  <link href="https://fonts.googleapis.com/css2?family=Pangolin&display=swap" rel="stylesheet">
  <style>
    /* 全局样式 */
    body {
      margin: 0;
      padding: 0;
      font-family: 'Pangolin', cursive, sans-serif;
      background-size: cover;
      background-position: center;
      background-attachment: fixed;
      overflow-x: hidden;
      display: flex;
      flex-direction: column;
      min-height: 100vh;
      background-color: #fffafc;
    }
    .page-wrapper {
      display: flex;
      flex-wrap: wrap;
      justify-content: center;
      align-items: flex-start;
      padding: 20px;
    }
    .ad-space {
      width: 160px;
      padding: 10px;
    }
    @media (max-width: 768px) {
      .ad-space {
        display: none;
      }
    }
    .container {
      flex: 1;
      width: 100%;
      max-width: 800px;
      margin: 20px;
      padding: 30px;
      background-color: rgba(255, 255, 255, 0.95);
      backdrop-filter: blur(5px);
      border-radius: 25px;
      box-shadow: 0 2px 20px rgba(0, 0, 0, 0.1);
      position: relative;
    }
    h1, h2 {
      text-align: center;
      text-shadow: 2px 2px 4px #FFD1DC;
      color: #FF6F91;
      margin: 10px 0;
    }
    h1 {
      font-size: 2.5em;
    }
    h2 {
      font-size: 1.8em;
    }
    @media (max-width: 768px) {
      h1 {
        font-size: 2em;
      }
      h2 {
        font-size: 1.5em;
      }
    }
    label {
      font-size: 1.2em;
      letter-spacing: 1px;
      display: block;
      margin-bottom: 5px;
      color: #FF6F91;
    }
    input, textarea {
      background-color: #fff;
      color: #2F6F91;
      border: 2px solid #FFD1DC;
      padding: 12px;
      margin: 5px 0 15px 0;
      border-radius: 15px;
      width: 100%;
      box-sizing: border-box;
      font-size: 1em;
    }
    input::placeholder, textarea::placeholder {
      color: #FF6F91;
    }
    .button-container {
      display: flex;
      flex-wrap: wrap;
      justify-content: space-between;
      margin-top: 20px;
    }
    button {
      background-color: #FFB6C1;
      color: #FFF;
      border: none;
      padding: 15px 30px;
      cursor: pointer;
      transition: all 0.3s ease;
      border-radius: 25px;
      letter-spacing: 1px;
      font-weight: bold;
      box-shadow: 0 0 15px rgba(255, 182, 193, 0.7);
      flex-grow: 1;
      margin: 10px 5px;
    }
    button:hover {
      background-color: #FF91A4;
      transform: scale(1.05);
      box-shadow: 0 0 25px rgba(255, 182, 193, 0.7);
    }
    button:active {
      transform: scale(0.95);
    }
    #loading, #error, #success {
      padding: 15px;
      margin: 15px 0;
      border: 2px solid;
      border-radius: 15px;
      text-align: center;
      font-size: 1.2em;
    }
    #error {
      color: #FF4500;
      border-color: #FF4500;
      background-color: #FFF0F0;
    }
    #success {
      color: #2F6F91;
      border-color: #FFB6C1;
      background-color: #F9F9F9;
      animation: pulse 2s infinite;
    }
    @keyframes pulse {
      0% { box-shadow: 0 0 0 0 rgba(255, 182, 193, 0.7); }
      70% { box-shadow: 0 0 0 15px rgba(255, 182, 193, 0); }
      100% { box-shadow: 0 0 0 0 rgba(255, 182, 193, 0); }
    }
    a {
      color: #FF6F91;
      text-decoration: none;
      border-bottom: 2px dashed #FF6F91;
      transition: all 0.3s;
      word-break: break-all;
    }
    a:hover {
      border-bottom: 2px solid #FF6F91;
      color: #FF1493;
    }
    #content-container {
      background-color: rgba(255, 182, 193, 0.3);
      border-radius: 15px;
      padding: 20px;
      margin-top: 25px;
    }
    hr {
      border: none;
      border-top: 3px dashed #FFB6C1;
      margin: 25px 0;
    }
    footer {
      text-align: center;
      padding: 20px;
      background-color: rgba(255, 111, 145, 0.8);
      border-top: 3px solid #FF6F91;
      font-family: sans-serif;
      font-size: 1.2em;
      letter-spacing: 1px;
      color: #FFFFFF;
    }
    .cat-paw {
      width: 40px;
      height: 40px;
      background-color: #FFB6C1;
      border-radius: 50%;
      display: inline-block;
      margin: 0 10px;
      position: relative;
      animation: wave 3s infinite;
      box-shadow: 0 0 10px rgba(255, 111, 145, 0.7);
    }
    @keyframes wave {
      0%, 100% { transform: rotate(0deg); }
      25% { transform: rotate(20deg); }
      75% { transform: rotate(-20deg); }
    }
    .floating-icon {
      position: fixed;
      bottom: 20px;
      right: 20px;
      font-size: 2em;
      color: #FF6F91;
    }
    /* 移动端优化 */
    @media (max-width: 480px) {
      .container {
        padding: 20px;
      }
      h1, h2 {
        font-size: 1.8em;
      }
      label, input, textarea, button {
        font-size: 1em;
      }
      .floating-icon {
        font-size: 1.5em;
        bottom: 15px;
        right: 15px;
      }
    }
  </style>
  <!-- 引入 OpenPGP.js 库 -->
  <script src="https://unpkg.com/openpgp@5.5.0/dist/openpgp.min.js"></script>
</head>
<body>
  <div class="page-wrapper">
    <div class="ad-space"></div>
    <div class="container">
      <div class="animated-background"></div>
      <h1 class="animate__animated animate__fadeInDown">
        <span class="cat-icon">🐱</span> Nyaa~crypted Kitty Note <span class="cat-icon">🐱</span>
      </h1>
      <h2 class="animate__animated animate__fadeInDown animate__delay-1s">// New purr-mission</h2>
      <form class="form animate__animated animate__fadeInUp animate__delay-2s" action="javascript:void(0);" method="post">
        <fieldset class="form-group form-textarea">
          <label for="message"><i class="fas fa-comment-dots"></i> ENCRYPTED_NYAA:</label>
          <textarea id="message" name="message" rows="10" placeholder="Enter your classified cat-formation here..." class="form-control"></textarea>
        </fieldset>
        <fieldset class="form-group" style="display: none;">
          <label for="fileInput"><i class="fas fa-file-upload"></i> ENCRYPTED_PAW_PRINT:</label>
          <input type="file" id="fileInput" name="fileInput" class="form-control">
        </fieldset>
        <div class="button-container">
          <button type="button" id="switchType"><i class="fas fa-exchange-alt"></i> SWITCH_NYAA_MODE</button>
          <button type="submit"><i class="fas fa-lock"></i> ENCRYPT_AND PURR-TRANSMIT</button>
        </div>
      </form>
      <div id="loading" style="display: none;">ENCRYPTING_WHISKERS...</div>
      <div id="error" style="display: none;" class="alert alert-error"></div>
      <div id="success" style="display: none;" class="alert alert-success">
        SECURE_CATWALK: <a id="link" href=""></a>
      </div>
      <hr>
      <ol>
        <li>CREATE_ENCRYPTED_NYAA</li>
        <li>TRANSMIT_SECURE_PURR-LINK</li>
        <li>MESSAGE_SELF_DESTRUCTS_AFTER_READING</li>
      </ol>
      <div id="content-container" class="alert alert-info">
        <div id="content"></div>
      </div>
    </div>
    <div class="ad-space"></div>
  </div>
  <footer>
    <div class="cat-paw"></div>
    <div class="cat-paw"></div>
    <div class="cat-paw"></div>
    Theme by Anon_Neko
    <div class="cat-paw"></div>
    <div class="cat-paw"></div>
    <div class="cat-paw"></div>
  </footer>
  <!-- 浮动图标 -->
  <div class="floating-icon"><i class="fas fa-cat"></i></div>
  <script>
    document.addEventListener('DOMContentLoaded', function() {
      const switchType = document.getElementById('switchType');
      const messageField = document.querySelector('fieldset.form-textarea');
      const messageInput = document.getElementById('message');
      const fileInput = document.getElementById('fileInput');
      const fileField = fileInput.parentElement;

      // 获取随机二次元背景图片
      fetch('https://api.waifu.pics/sfw/waifu')
        .then(response => response.json())
        .then(data => {
          document.body.style.backgroundImage = "url('" + data.url + "')";
        })
        .catch(error => console.error('Error fetching background image:', error));

      switchType.addEventListener('click', function() {
        if (messageField.style.display === 'none') {
          messageField.style.display = 'block';
          fileField.style.display = 'none';
          this.innerHTML = '<i class="fas fa-file-image"></i> SWITCH_TO_PAW_PRINT_MODE';
        } else {
          messageField.style.display = 'none';
          fileField.style.display = 'block';
          this.innerHTML = '<i class="fas fa-comment-dots"></i> SWITCH_TO_MEOW_MODE';
        }
      });

      // 检查 Web Crypto API 支持
      if (!window.crypto || !window.crypto.subtle) {
        console.error("Web Crypto API not supported");
        alert("喵呜~ 你的浏览器不支持所需的加密功能。请使用现代浏览器！");
        return;
      }

      async function generateKey() {
        try {
          console.log("Generating encryption key...");
          const key = await window.crypto.subtle.generateKey(
            { name: "AES-GCM", length: 256 },
            true,
            ["encrypt", "decrypt"]
          );
          console.log("Encryption key generated successfully");
          return key;
        } catch (error) {
          console.error("Error generating key:", error);
          throw new Error("喵呜~ 生成加密密钥失败");
        }
      }

      async function encryptFile(file) {
        try {
          console.log("Starting file encryption...");
          const key = await generateKey();
          const iv = window.crypto.getRandomValues(new Uint8Array(12));
          const fileData = await file.arrayBuffer();
          console.log("Encrypting file data...");
          const encryptedContent = await window.crypto.subtle.encrypt(
            { name: "AES-GCM", iv: iv },
            key,
            fileData
          );
          console.log("Exporting key...");
          const exportedKey = await window.crypto.subtle.exportKey("raw", key);
          console.log("File encryption completed");
          return { encryptedContent, iv, exportedKey };
        } catch (error) {
          console.error("Error encrypting file:", error);
          throw new Error("喵呜~ 加密文件失败");
        }
      }

      async function encryptAndUploadFile(file) {
        try {
          console.log("Starting file encryption and upload process...");
          const { encryptedContent, iv, exportedKey } = await encryptFile(file);
          const formData = new FormData();
          formData.append('file', new Blob([encryptedContent]), file.name + '.enc');
          formData.append('fileName', file.name);
          formData.append('fileType', file.type);
          formData.append('iv', new Blob([iv]));
          formData.append('key', new Blob([exportedKey]));
          console.log("Sending encrypted file to server...");
          const response = await fetch('/save-file', {
            method: 'POST',
            body: formData
          });
          if (!response.ok) {
            const errorText = await response.text();
            throw new Error("HTTP error! status: " + response.status + ", message: " + errorText);
          }
          const result = await response.json();
          if (!result.filename) {
            throw new Error("喵呜~ 服务器没有返回文件名");
          }
          return { id: result.filename, iv, exportedKey };
        } catch (error) {
          console.error("Error uploading file:", error);
          throw new Error("喵呜~ 上传文件失败: " + error.message);
        }
      }

      async function downloadAndDecryptFile(fileId, keyData) {
        try {
          console.log('Starting file download and decryption');
          const response = await fetch("/get-file?id=" + encodeURIComponent(fileId));
          if (!response.ok) {
            let errorMessage = "HTTP error! status: " + response.status;
            try {
              const errorData = await response.json();
              errorMessage += ", message: " + (errorData.error || 'Unknown error');
            } catch (e) {
              console.error('Failed to parse error response:', e);
            }
            throw new Error(errorMessage);
          }
          const data = await response.json();
          if (!data.encryptedFile) {
            throw new Error('喵呜~ 没有收到加密的文件数据');
          }
          const key = await window.crypto.subtle.importKey(
            "raw",
            new Uint8Array(keyData.key),
            { name: "AES-GCM", length: 256 },
            false,
            ["decrypt"]
          );
          const encryptedData = new Uint8Array(atob(data.encryptedFile).split('').map(char => char.charCodeAt(0)));
          const decryptedContent = await window.crypto.subtle.decrypt(
            { name: "AES-GCM", iv: new Uint8Array(keyData.iv) },
            key,
            encryptedData
          );
          const blob = new Blob([decryptedContent], { type: data.fileType });
          const url = URL.createObjectURL(blob);
          const a = document.createElement('a');
          a.href = url;
          a.download = data.fileName || 'downloaded_file';
          document.body.appendChild(a);
          a.click();
          document.body.removeChild(a);
          URL.revokeObjectURL(url);
          // 文件成功解密后，发送销毁请求
          const burnResponse = await fetch("/burn-file?id=" + encodeURIComponent(fileId), { method: 'POST' });
          if (!burnResponse.ok) {
            console.warn('Failed to burn file:', await burnResponse.text());
          }
          document.getElementById('content').innerText = '喵呜~ 文件已成功下载，并已从服务器删除！';
        } catch (error) {
          let errorMessage = '喵呜~ 下载或解密文件时出现错误：';
          if (error.message.includes("File has been burned")) {
            errorMessage = '喵呜~ 文件已经被销毁了！';
          } else if (error.message.includes("HTTP error!")) {
            errorMessage += error.message;
          } else {
            errorMessage += error.toString();
          }
          document.getElementById('content').innerText = errorMessage;
        }
      }

      document.querySelector('form').addEventListener('submit', async function(e) {
        e.preventDefault();
        document.getElementById('loading').style.display = 'block';
        document.getElementById('error').style.display = 'none';
        document.getElementById('success').style.display = 'none';
        try {
          const isFileMode = messageField.style.display === 'none';
          let id, key;
          if (isFileMode) {
            const file = fileInput.files[0];
            if (!file) throw new Error('喵呜~ 请选择一个文件。');
            if (file.size > 15 * 1024 * 1024) {
              throw new Error('喵呜~ 文件大小不能超过15MB。');
            }
            const { id: fileId, iv, exportedKey } = await encryptAndUploadFile(file);
            id = fileId;
            key = btoa(JSON.stringify({ iv: Array.from(iv), key: Array.from(new Uint8Array(exportedKey)) }));
          } else {
            const message = messageInput.value;
            if (!message) throw new Error('喵呜~ 请输入一条消息。');
            const keyPair = await openpgp.generateKey({
              type: 'ecc',
              curve: 'curve25519',
              userIDs: [{ name: 'Anonymous', email: 'anonymous@example.com' }]
            });
            const publicKey = await openpgp.readKey({ armoredKey: keyPair.publicKey });
            const privateKey = await openpgp.readKey({ armoredKey: keyPair.privateKey });
            const encrypted = await openpgp.encrypt({
              message: await openpgp.createMessage({ text: message }),
              encryptionKeys: publicKey
            });
            const response = await fetch('/save-message', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ message: encrypted })
            });
            if (!response.ok) {
              const errorText = await response.text();
              if (errorText.startsWith('<')) {
                throw new Error('喵呜~ 服务器错误: ' + errorText);
              } else {
                const errorData = JSON.parse(errorText);
                throw new Error('喵呜~ 错误: ' + errorData.error);
              }
            }
            const result = await response.json();
            if (result.error) throw new Error(result.error);
            id = result.id;
            key = btoa(privateKey.armor());
          }
          const type = isFileMode ? 'file' : 'message';
          const longLink = window.location.origin + window.location.pathname + '?id=' + id + '&key=' + key + '&type=' + type;
          document.getElementById('link').href = longLink;
          document.getElementById('link').innerText = '喵呜~ 正在生成链接，请稍等...';
          document.getElementById('success').style.display = 'block';
          const response = await fetch('/generate-short-link', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: 'longUrl=' + encodeURIComponent(longLink)
          });
          const data = await response.json();
          if (data.error) {
            throw new Error(data.error);
          }
          const shortLink = data.shortUrl;
          document.getElementById('link').href = shortLink;
          document.getElementById('link').innerText = shortLink;
        } catch (error) {
          document.getElementById('error').innerText = '喵呜~ 出错了: ' + error.message;
          document.getElementById('error').style.display = 'block';
        } finally {
          document.getElementById('loading').style.display = 'none';
        }
      });

      if (new URLSearchParams(window.location.search).has('id') &&
          new URLSearchParams(window.location.search).has('key') &&
          new URLSearchParams(window.location.search).has('type')) {
        const id = new URLSearchParams(window.location.search).get('id');
        const key = new URLSearchParams(window.location.search).get('key');
        const type = new URLSearchParams(window.location.search).get('type');
        if (type === 'file') {
          try {
            const keyData = JSON.parse(atob(key));
            if (!id || id === 'undefined') {
              throw new Error('喵呜~ 无效的文件ID');
            }
            downloadAndDecryptFile(id, keyData);
          } catch (error) {
            document.getElementById('content').innerText = '喵呜~ 解析密钥数据时出错：' + error.message;
          }
        } else if (type === 'message') {
          fetch('/get-message?id=' + id)
            .then(response => {
              if (!response.ok) throw new Error('HTTP error! status: ' + response.status);
              return response.json();
            })
            .then(async data => {
              if (data.message === "The message has been burned!") {
                document.getElementById('content').innerText = '喵呜~ ' + data.message;
              } else if (data.error) {
                throw new Error(data.error);
              } else {
                const privateKey = await openpgp.readPrivateKey({ armoredKey: atob(key) });
                const message = await openpgp.readMessage({ armoredMessage: data.message });
                const { data: decrypted } = await openpgp.decrypt({
                  message,
                  decryptionKeys: privateKey
                });
                document.getElementById('content').innerText = decrypted;
              }
            })
            .catch(error => {
              document.getElementById('content').innerText = '喵呜~ 错误: ' + error.message;
            });
        } else {
          document.getElementById('content').innerText = '喵呜~ 无效的类型参数。';
        }
      }
    });
  </script>
</body>
</html>

`

func main() {
	// Parse command line flags
	configFile := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	config, err := LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Database check removed
	// Use 0.0.0.0 to bind to all interfaces inside the container, or use config value if needed
	host := "0.0.0.0" // Or use config.Server.Host if you want it configurable
	port := strconv.Itoa(config.Server.Port)
	log.Printf("Server running on %s:%s", host, port)

	// No database initialization needed

	router := gin.Default()

	// Check storage permissions
	if err := CheckStoragePermissions(); err != nil {
		log.Fatalf("Storage permissions check failed: %v", err)
	}

	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}

	router.Use(cors.New(corsConfig))

	// 处理根路径，返回嵌入的HTML内容
	router.GET("/", func(c *gin.Context) {
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, indexHTML)
	})

	router.POST("/save-message", saveMessage)
	router.POST("/save-file", SaveFileHandler)
	router.GET("/get-message", getMessage)
	router.GET("/get-file", getFile)
	router.POST("/generate-short-link", generateShortLink)
	router.GET("/s/:shortCode", redirect)

	// 普通HTTP模式
	log.Printf("Server running HTTP on %s:%s", host, port)
	if err := router.Run(fmt.Sprintf("%s:%s", host, port)); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}
