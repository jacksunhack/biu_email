package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings" // Import strings package

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	// _ "github.com/go-sql-driver/mysql" // Keep commented or remove if not needed
)

// indexHTML 包含完整的前端代码
const indexHTML = `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Biu~ 阅后即焚 (客户端加密版)</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; line-height: 1.6; padding: 20px; max-width: 700px; margin: auto; background-color: #f8f9fa; color: #343a40; }
        .container { background: #ffffff; padding: 30px; border-radius: 8px; box-shadow: 0 4px 12px rgba(0,0,0,0.08); }
        h1, h2 { color: #495057; text-align: center; margin-bottom: 1.5rem;}
        h1 { font-size: 2rem;}
        h2 { font-size: 1.5rem; color: #6c757d;}
        textarea, input[type="file"], input[type="text"] { width: 100%; padding: 12px; margin-bottom: 15px; border: 1px solid #ced4da; border-radius: 4px; box-sizing: border-box; font-size: 1rem; }
        textarea { min-height: 120px; resize: vertical; }
        button { background-color: #007bff; color: white; padding: 12px 20px; border: none; border-radius: 4px; cursor: pointer; font-size: 1rem; margin-right: 10px; transition: background-color 0.2s ease-in-out; }
        button:hover { background-color: #0056b3; }
        #switchType { background-color: #ffc107; color: #212529;}
        #switchType:hover { background-color: #e0a800; }
        #result, #content-area, #status { margin-top: 20px; padding: 15px; border-radius: 4px; font-size: 0.95rem; }
        #result { background-color: #d1ecf1; border: 1px solid #bee5eb; color: #0c5460; word-wrap: break-word; }
        #result a { color: #0c5460; font-weight: bold; text-decoration: none; }
        #result a:hover { text-decoration: underline; }
        #content-area { background-color: #e9ecef; border: 1px solid #dee2e6; color: #495057; white-space: pre-wrap; word-wrap: break-word; min-height: 100px; }
        #status { background-color: #fff3cd; border: 1px solid #ffeeba; color: #856404; }
        #error { background-color: #f8d7da; border: 1px solid #f5c6cb; color: #721c24; }
        .hidden { display: none; }
        label { display: block; margin-bottom: 5px; font-weight: 500; color: #495057;}
        .form-group { margin-bottom: 1rem; }
        .button-container { display: flex; justify-content: flex-start; align-items: center; margin-top: 1.5rem; }
        .loader { border: 4px solid #f3f3f3; border-radius: 50%; border-top: 4px solid #007bff; width: 20px; height: 20px; animation: spin 1s linear infinite; display: inline-block; vertical-align: middle; margin-left: 10px;}
        @keyframes spin { 0% { transform: rotate(0deg); } 100% { transform: rotate(360deg); } }
        footer { text-align: center; margin-top: 30px; padding-top: 20px; border-top: 1px solid #dee2e6; font-size: 0.9em; color: #6c757d; }
        footer a { color: #007bff; text-decoration: none; }
        footer a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Biu~ 阅后即焚</h1>
        <h2>(客户端加密版)</h2>

        <div class="form-group">
            <label for="contentType">内容类型:</label>
            <button id="switchType">切换到文件模式</button>
        </div>

        <form id="encryptForm">
            <div id="textMode">
                <div class="form-group">
                    <label for="message">输入文本:</label>
                    <textarea id="message" rows="8" placeholder="在此输入你的秘密消息..."></textarea>
                </div>
            </div>
            <div id="fileMode" class="hidden">
                <div class="form-group">
                    <label for="fileInput">选择文件:</label>
                    <input type="file" id="fileInput">
                    <small id="fileSizeWarning" class="hidden" style="color: red;">文件大小超过限制！</small>
                </div>
            </div>
            <div class="button-container">
                <button type="submit" id="submitBtn">加密并生成链接</button>
                <div id="loader" class="loader hidden"></div>
            </div>
        </form>

        <div id="status" class="hidden"></div>
        <div id="error" class="hidden"></div>
        <div id="result" class="hidden">
            <p>成功！你的阅后即焚链接：</p>
            <a id="link" href="#" target="_blank"></a>
            <p><small>请注意：此链接仅能访问一次，密钥存储在 # 之后的部分，不会发送到服务器。</small></p>
        </div>

        <div id="content-area" class="hidden">
            <h2>解密内容:</h2>
            <div id="decrypted-content"></div>
        </div>
    </div>

    <footer>
        Powered by Go & Gin | Client-Side Encryption with AES-GCM + HKDF
        <br>
        <a href="https://github.com/jacksunhack/biu_email" target="_blank">GitHub Repository</a>
    </footer>

    <script>
        let isFileMode = false;
        let maxFileSizeMB = 15; // Default, will be updated from config

        // --- DOM Elements ---
        const switchTypeButton = document.getElementById('switchType');
        const textModeDiv = document.getElementById('textMode');
        const fileModeDiv = document.getElementById('fileMode');
        const messageTextarea = document.getElementById('message');
        const fileInput = document.getElementById('fileInput');
        const fileSizeWarning = document.getElementById('fileSizeWarning');
        const encryptForm = document.getElementById('encryptForm');
        const submitBtn = document.getElementById('submitBtn');
        const loader = document.getElementById('loader');
        const statusDiv = document.getElementById('status');
        const errorDiv = document.getElementById('error');
        const resultDiv = document.getElementById('result');
        const linkElement = document.getElementById('link');
        const contentAreaDiv = document.getElementById('content-area');
        const decryptedContentDiv = document.getElementById('decrypted-content');

        // --- Utility Functions ---
        function arrayBufferToBase64(buffer) {
            let binary = '';
            const bytes = new Uint8Array(buffer);
            const len = bytes.byteLength;
            for (let i = 0; i < len; i++) {
                binary += String.fromCharCode(bytes[i]);
            }
            return window.btoa(binary);
        }

        function base64ToArrayBuffer(base64) {
            const binary_string = window.atob(base64);
            const len = binary_string.length;
            const bytes = new Uint8Array(len);
            for (let i = 0; i < len; i++) {
                bytes[i] = binary_string.charCodeAt(i);
            }
            return bytes.buffer;
        }

        function showStatus(message, isError = false) {
            hideMessages();
            const targetDiv = isError ? errorDiv : statusDiv;
            targetDiv.textContent = message;
            targetDiv.classList.remove('hidden');
        }

        function showResult(url) {
            hideMessages();
            linkElement.href = url;
            linkElement.textContent = url;
            resultDiv.classList.remove('hidden');
        }

        function hideMessages() {
            statusDiv.classList.add('hidden');
            errorDiv.classList.add('hidden');
            resultDiv.classList.add('hidden');
            contentAreaDiv.classList.add('hidden');
        }

        function setLoading(isLoading) {
            if (isLoading) {
                loader.classList.remove('hidden');
                submitBtn.disabled = true;
                submitBtn.textContent = '处理中...';
            } else {
                loader.classList.add('hidden');
                submitBtn.disabled = false;
                submitBtn.textContent = '加密并生成链接';
            }
        }

        // --- Crypto Functions ---
        async function generateMasterKey() {
            const keyBytes = window.crypto.getRandomValues(new Uint8Array(32)); // 256 bits
            return arrayBufferToBase64(keyBytes); // Store master key as base64
        }

        async function deriveEncryptionKey(masterKeyBase64, salt) {
            const masterKey = base64ToArrayBuffer(masterKeyBase64);
            const keyMaterial = await window.crypto.subtle.importKey(
                "raw",
                masterKey,
                { name: "HKDF" },
                false,
                ["deriveKey"]
            );
            return window.crypto.subtle.deriveKey(
                {
                    name: "HKDF",
                    salt: salt,
                    info: new TextEncoder().encode("AES-GCM Encryption Key"), // Context info
                    hash: "SHA-256"
                },
                keyMaterial,
                { name: "AES-GCM", length: 256 },
                true, // Allow export for debugging if needed, set to false in prod
                ["encrypt", "decrypt"]
            );
        }

        async function encryptData(dataBuffer, encryptionKey) {
            const iv = window.crypto.getRandomValues(new Uint8Array(12)); // 96 bits is recommended for AES-GCM
            const encryptedContent = await window.crypto.subtle.encrypt(
                {
                    name: "AES-GCM",
                    iv: iv
                },
                encryptionKey,
                dataBuffer
            );
            return { encryptedContent, iv };
        }

        async function decryptData(encryptedBuffer, iv, encryptionKey) {
            try {
                const decryptedContent = await window.crypto.subtle.decrypt(
                    {
                        name: "AES-GCM",
                        iv: iv
                    },
                    encryptionKey,
                    encryptedBuffer
                );
                return decryptedContent;
            } catch (e) {
                console.error("Decryption failed:", e);
                throw new Error("解密失败，密钥或数据可能已损坏。");
            }
        }

        // --- Event Listeners ---
        switchTypeButton.addEventListener('click', () => {
            isFileMode = !isFileMode;
            if (isFileMode) {
                textModeDiv.classList.add('hidden');
                fileModeDiv.classList.remove('hidden');
                switchTypeButton.textContent = '切换到文本模式';
                switchTypeButton.style.backgroundColor = '#17a2b8'; // Info color
            } else {
                textModeDiv.classList.remove('hidden');
                fileModeDiv.classList.add('hidden');
                switchTypeButton.textContent = '切换到文件模式';
                switchTypeButton.style.backgroundColor = '#ffc107'; // Warning color
            }
            hideMessages(); // Clear status/results when switching modes
        });

        fileInput.addEventListener('change', () => {
            const file = fileInput.files[0];
            if (file) {
                const maxSizeBytes = maxFileSizeMB * 1024 * 1024;
                if (file.size > maxSizeBytes) {
                    fileSizeWarning.classList.remove('hidden');
                    submitBtn.disabled = true;
                } else {
                    fileSizeWarning.classList.add('hidden');
                    submitBtn.disabled = false;
                }
            } else {
                 fileSizeWarning.classList.add('hidden');
                 submitBtn.disabled = false;
            }
        });

        encryptForm.addEventListener('submit', async (event) => {
            event.preventDefault();
            hideMessages();
            setLoading(true);

            try {
                const masterKeyBase64 = await generateMasterKey();
                const salt = window.crypto.getRandomValues(new Uint8Array(16)); // Salt for HKDF
                const encryptionKey = await deriveEncryptionKey(masterKeyBase64, salt);

                let dataBuffer;
                let originalFilename = null; // Store filename only in file mode

                if (isFileMode) {
                    const file = fileInput.files[0];
                    if (!file) {
                        throw new Error("请选择一个文件。");
                    }
                    const maxSizeBytes = maxFileSizeMB * 1024 * 1024;
                     if (file.size > maxSizeBytes) {
                        throw new Error('文件大小超过限制 (' + maxFileSizeMB + ' MB)。');
                    }
                    showStatus("正在读取文件...");
                    dataBuffer = await file.arrayBuffer();
                    originalFilename = file.name; // Get filename
                    showStatus("正在加密文件...");
                } else {
                    const message = messageTextarea.value;
                    if (!message.trim()) {
                        throw new Error("请输入文本消息。");
                    }
                    dataBuffer = new TextEncoder().encode(message);
                    showStatus("正在加密文本...");
                }

                const { encryptedContent, iv } = await encryptData(dataBuffer, encryptionKey);

                const encryptedBase64 = arrayBufferToBase64(encryptedContent);
                const ivBase64 = arrayBufferToBase64(iv);
                const saltBase64 = arrayBufferToBase64(salt);

                showStatus("正在将加密数据发送到服务器...");

                const payload = {
                    encryptedData: encryptedBase64,
                    iv: ivBase64,
                    salt: saltBase64,
                    // Add originalFilename only if fileMode is true
                };
                if (isFileMode && originalFilename) {
                    payload.originalFilename = originalFilename;
                }


                const response = await fetch('/api/store', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload)
                });

                if (!response.ok) {
                    const errorData = await response.json().catch(() => ({ error: '无法解析服务器错误响应' }));
                    throw new Error('服务器错误 (' + response.status + '): ' + (errorData.error || response.statusText));
                }

                const resultData = await response.json();
                if (!resultData.id) {
                     throw new Error("服务器未能返回数据 ID。");
                }

                const dataId = resultData.id;
                const shareUrl = window.location.origin + window.location.pathname + '?id=' + dataId + '#' + masterKeyBase64;

                showResult(shareUrl);

            } catch (error) {
                console.error("加密/上传过程中出错:", error);
                showStatus('错误: ' + error.message, true);
            } finally {
                setLoading(false);
            }
        });

        // --- Decryption Logic (on page load if URL contains ID and Key) ---
        async function handleDecryptionOnLoad() {
            const urlParams = new URLSearchParams(window.location.search);
            const dataId = urlParams.get('id');
            const masterKeyBase64 = window.location.hash.substring(1); // Get key from URL fragment

            if (dataId && masterKeyBase64) {
                hideMessages();
                contentAreaDiv.classList.remove('hidden');
                decryptedContentDiv.innerHTML = '正在获取加密数据...';
                setLoading(true); // Use loader visually

                try {
                    const response = await fetch('/api/data/' + dataId);

                    if (!response.ok) {
                         if (response.status === 404) {
                             throw new Error("数据未找到或已被销毁。");
                         }
                         const errorData = await response.json().catch(() => ({ error: '无法解析服务器错误响应' }));
                         throw new Error('获取数据时服务器错误 (' + response.status + '): ' + (errorData.error || response.statusText));
                    }

                    const responseData = await response.json();
                    if (!responseData.encryptedData || !responseData.iv || !responseData.salt) {
                        throw new Error("从服务器接收到的数据不完整。");
                    }

                    decryptedContentDiv.innerHTML = '正在解密数据...';

                    const encryptedData = base64ToArrayBuffer(responseData.encryptedData);
                    const iv = base64ToArrayBuffer(responseData.iv);
                    const salt = base64ToArrayBuffer(responseData.salt);
                    const originalFilename = responseData.originalFilename; // Get filename if present

                    const encryptionKey = await deriveEncryptionKey(masterKeyBase64, salt);
                    const decryptedBuffer = await decryptData(encryptedData, iv, encryptionKey);

                    // Try to detect if it's text or file based on originalFilename
                    if (originalFilename) {
                        // Assume it's a file
                        decryptedContentDiv.innerHTML = '文件已解密: <strong>' + originalFilename + '</strong><br>准备下载...';
                        const blob = new Blob([decryptedBuffer]); // Use appropriate MIME type if available, otherwise default
                        const url = URL.createObjectURL(blob);
                        const a = document.createElement('a');
                        a.href = url;
                        // Use the original filename from the server response
                        const filenameToUse = originalFilename || ('decrypted_data_' + dataId + '.bin'); // Fallback if filename missing
                        a.download = filenameToUse;
                        console.log('Triggering download for: ' + filenameToUse); // Log the filename being used
                        document.body.appendChild(a);
                        a.click();
                        document.body.removeChild(a);
                        URL.revokeObjectURL(url);
                        decryptedContentDiv.innerHTML += "<br>下载已开始。文件将在下载后从服务器销毁。";

                    } else {
                        // Assume it's text
                        const decryptedText = new TextDecoder().decode(decryptedBuffer);
                        decryptedContentDiv.textContent = decryptedText;
                         decryptedContentDiv.innerHTML += "<br><br><small>消息将在销毁后从服务器删除。</small>";
                    }


                    // Burn the data after successful decryption/download attempt
                    try {
                        await fetch('/api/burn/' + dataId, { method: 'POST' });
                        console.log("Burn request sent for ID:", dataId);
                         // Optionally update UI to confirm burn, but download/display is primary
                    } catch (burnError) {
                        console.warn("发送销毁请求失败:", burnError);
                        // Inform user, but decryption was successful
                         decryptedContentDiv.innerHTML += "<br><strong style='color:orange;'>警告：无法自动销毁服务器上的数据。</strong>";
                    }

                } catch (error) {
                    console.error("解密/获取过程中出错:", error);
                    contentAreaDiv.classList.remove('hidden'); // Ensure area is visible for error
                    decryptedContentDiv.innerHTML = '<span style="color:red;">错误: ' + error.message + '</span>';
                } finally {
                     setLoading(false);
                }
            }
        }

         // --- Initial Setup ---
        document.addEventListener('DOMContentLoaded', async () => {
            console.log("DOM fully loaded.");
             // Fetch config first
            try {
                const configResponse = await fetch('/config');
                if (configResponse.ok) {
                    const configData = await configResponse.json();
                    if (configData.maxFileSizeMB) {
                        maxFileSizeMB = parseInt(configData.maxFileSizeMB, 10);
                        console.log('Max file size loaded from config:', maxFileSizeMB, 'MB');
                    } else {
                         console.warn('Config endpoint did not return maxFileSizeMB.');
                    }
                } else {
                    console.warn('Failed to load config from server, using default max file size:', maxFileSizeMB, 'MB');
                }
            } catch (error) {
                console.error('Error fetching config:', error);
                 showStatus('无法加载服务器配置: ' + error.message, true);
            }

            // Then handle decryption if needed
            handleDecryptionOnLoad();
        });

    </script>
</body>
</html>
\`

var config *Config // Global config variable

func main() {
	// Parse command line flags
	configFile := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	var err error
	config, err = LoadConfig(*configFile) // Assign to global config
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Ensure data storage directory exists (moved from api_handlers.go for clarity)
	if err := ensureDataStorageDir(); err != nil {
		log.Fatalf("Failed to ensure data storage directory exists: %v", err)
	}

	// Use 0.0.0.0 to bind to all interfaces inside the container, or use config value if needed
	host := "0.0.0.0" // Or use config.Server.Host if you want it configurable
	port := strconv.Itoa(config.Server.Port)
	log.Printf("Server running on %s:%s", host, port)

	// 设置 Gin 为 release 模式以提高性能
	gin.SetMode(gin.ReleaseMode)
	// Disable Gin's default logging to avoid duplicate timestamps if using custom log
	// router := gin.New()
	// router.Use(gin.Recovery()) // Use recovery middleware
	router := gin.Default() // Or keep default if its logging is acceptable

	// CORS Configuration
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true // Be careful in production, restrict if possible
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}

	router.Use(cors.New(corsConfig))

	// 处理根路径，返回嵌入的HTML内容
	router.GET("/", func(c *gin.Context) {
		// Check if the request is for the root path specifically
		// Also handle direct access with ID and fragment (key) for decryption page
		if (c.Request.URL.Path == "/" && c.Request.URL.RawQuery == "" && c.Request.URL.Fragment == "") ||
			(strings.HasPrefix(c.Request.URL.RawQuery, "id=") && c.Request.URL.Fragment != "") {
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.String(http.StatusOK, indexHTML) // Serve the same HTML, JS will handle decryption
		} else {
			// Optional: Handle other paths or return 404
			// For simplicity, we can also just serve indexHTML for any GET request
			// that isn't an API endpoint, letting the JS handle routing/display.
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.String(http.StatusOK, indexHTML)
			// Alternatively, return 404:
			// c.String(http.StatusNotFound, "404 Not Found")
		}
	})

	// --- New API Endpoints ---
	router.POST("/api/store", StoreDataHandler)   // Stores encrypted data, returns ID
	router.GET("/api/data/:id", GetDataHandler)   // Gets encrypted data, IV, Salt by ID
	router.POST("/api/burn/:id", BurnDataHandler) // Burns data by ID

	// --- 返回配置信息的端点 (保持) ---
	router.GET("/config", func(c *gin.Context) {
		if config == nil {
			// 确保 config 已加载
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server configuration not loaded"})
			return
		}
		// 只返回前端需要的信息
		c.JSON(http.StatusOK, gin.H{
			"maxFileSizeMB": config.Server.MaxFileSizeMB,
		})
	})
	// --- 结束新增 ---

	// 普通HTTP模式
	log.Printf("Server running HTTP on %s:%s", host, port)
	if err := router.Run(fmt.Sprintf("%s:%s", host, port)); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}

// ensureDataStorageDir ensures the directory for storing data files exists.
// It's defined here to be accessible by main.
func ensureDataStorageDir() error {
	dataDir := filepath.Join("storage", "data") // Consistent path
	// Use MkdirAll which creates parent directories if needed and doesn't return error if dir exists
	if err := os.MkdirAll(dataDir, 0750); err != nil { // Use 0750 for better permissions
		log.Printf("Error creating data storage directory '%s': %v", dataDir, err)
		return fmt.Errorf("failed to create data storage directory: %w", err)
	}
	log.Printf("Data storage directory ensured: %s", dataDir)
	return nil
}
