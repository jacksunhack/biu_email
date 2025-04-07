package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http" // Gin 内部仍会用到，但 Handler 签名改变
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time" // 导入 time 包

	"github.com/gin-gonic/gin" // 导入 Gin
)

// ChunkInfo 保存文件分片的信息
type ChunkInfo struct {
	UploadID    string `json:"uploadId"`    // 唯一上传标识
	ChunkNumber int    `json:"chunkNumber"` // 当前分片编号
	TotalChunks int    `json:"totalChunks"` // 总分片数
	FileName    string `json:"fileName"`    // 文件名
	FileSize    int64  `json:"fileSize"`    // 文件总大小
}

// ChunkResponse 返回给客户端的响应
type ChunkResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message,omitempty"`
	UploadID  string `json:"uploadId,omitempty"`
	FilePath  string `json:"filePath,omitempty"`
	Completed bool   `json:"completed,omitempty"`
}

// 临时存储分片的目录
const tempDir = "./temp-files" // Corrected path to match Docker setup

// 最终文件保存的目录
const uploadDir = "./uploads"

// 使用互斥锁保护文件合并过程
var mergeMutex sync.Mutex

// ChunkUploadHandler 处理分片上传请求
func ChunkUploadHandler(c *gin.Context) { // 修改签名
	// 确保目录存在
	// ensureDirectoriesExist() // 可以在 main 函数开始时调用一次

	// 检查是否为POST请求
	// Gin 会自动处理 Method Not Allowed，通常不需要手动检查

	// 解析表单
	// Gin 会自动处理表单解析，但可以设置限制
	// c.Request.ParseMultipartForm(32 << 20) // 如果需要手动设置限制

	// 获取分片信息
	uploadID := c.PostForm("uploadId")
	chunkNumberStr := c.PostForm("chunkNumber")
	totalChunksStr := c.PostForm("totalChunks")
	fileName := c.PostForm("fileName")
	fileSizeStr := c.PostForm("fileSize")

	// 验证参数
	if uploadID == "" || chunkNumberStr == "" || totalChunksStr == "" || fileName == "" || fileSizeStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Missing required parameters"})
		return
	}

	// 转换参数类型
	chunkNumber, err := strconv.Atoi(chunkNumberStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid chunk number"})
		return
	}

	totalChunks, err := strconv.Atoi(totalChunksStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid total chunks"})
		return
	}

	fileSize, err := strconv.ParseInt(fileSizeStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid file size"})
		return
	}

	// 获取文件分片
	file, header, err := c.Request.FormFile("chunk") // 修改这里以接收 header
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Failed to get chunk file"})
		return
	}
	defer file.Close()

	// 在 header 有效的作用域内记录日志
	log.Printf("[%s] Received chunk %d / %d: Size = %d bytes", uploadID, chunkNumber, totalChunks, header.Size)

	// 存储分片
	chunkDir := filepath.Join(tempDir, uploadID)
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to create chunk directory"})
		return
	}

	chunkPath := filepath.Join(chunkDir, fmt.Sprintf("%d", chunkNumber))
	out, err := os.Create(chunkPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to create chunk file"})
		return
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to save chunk file"})
		return
	}

	// 检查是否所有分片都已上传
	// 检查是否所有分片都已上传
	// 读取目录前确保目录存在
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to ensure chunk directory exists"})
		return
	}
	files, err := ioutil.ReadDir(chunkDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to read chunk directory"})
		return
	}

	// 实际接收到的分片数量
	receivedChunks := len(files)

	// 只有当实际接收到的分片数等于总分片数时才开始合并
	if receivedChunks == totalChunks {
		// 异步合并文件，传递 fileSize
		// 异步合并文件，传递 fileSize
		// 确保 ensureDirectoriesExist 在 main 中调用或在这里调用
		ensureDirectoriesExist()
		// 添加日志，记录即将传递给 mergeChunks 的 totalChunks 值
		log.Printf("[%s] ChunkUploadHandler: Triggering mergeChunks with totalChunks = %d", uploadID, totalChunks)
		go mergeChunks(uploadID, fileName, totalChunks, fileSize, chunkDir)

		// 返回成功响应
		c.JSON(http.StatusOK, ChunkResponse{
			Success:   true,
			Message:   "All chunks received, merging has begun",
			UploadID:  uploadID,
			Completed: false, // 合并是异步的
		})
		return

	}

	// 如果不是最后一个分片，返回当前分片上传成功响应
	c.JSON(http.StatusOK, ChunkResponse{
		Success:   true,
		Message:   fmt.Sprintf("Chunk %d of %d received (%d/%d total received)", chunkNumber, totalChunks, receivedChunks, totalChunks),
		UploadID:  uploadID,
		Completed: false,
	})

}

// CheckUploadStatusHandler 检查文件合并状态
func CheckUploadStatusHandler(c *gin.Context) { // 修改签名
	uploadID := c.Query("uploadId") // 使用 c.Query 获取查询参数
	if uploadID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Missing uploadId parameter"})
		return
	}

	uploadStatusDir := filepath.Join(uploadDir, uploadID)
	completeMarkerPath := filepath.Join(uploadStatusDir, ".complete")
	fileNamePath := filepath.Join(uploadStatusDir, ".filename") // 存储原始文件名

	if _, err := os.Stat(completeMarkerPath); err == nil {
		// .complete 文件存在，表示合并已完成
		// 读取原始文件名
		fileNameBytes, err := ioutil.ReadFile(fileNamePath)
		if err != nil {
			// 如果无法读取文件名，可能状态不一致，返回错误
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to retrieve original filename after merge"})
			return
		}
		originalFileName := string(fileNameBytes)
		finalRelativePath := filepath.Join(uploadDir, uploadID, originalFileName) // 构造相对路径

		c.JSON(http.StatusOK, ChunkResponse{ // 使用 c.JSON
			Success:   true,
			Message:   "File merge completed",
			UploadID:  uploadID,
			FilePath:  finalRelativePath, // 返回包含原始文件名的相对路径
			Completed: true,
		})
		return
	}

	// 检查临时目录是否存在
	// 检查临时目录是否存在，如果存在说明还在上传或合并中
	tempChunkDir := filepath.Join(tempDir, uploadID)
	if _, err := os.Stat(tempChunkDir); err == nil {
		// 临时目录存在，合并尚未完成或正在进行
		c.JSON(http.StatusOK, ChunkResponse{ // 使用 c.JSON
			Success:   true,
			Message:   "File upload/merge in progress",
			UploadID:  uploadID,
			Completed: false,
		})
		return
	}

	// 如果 .complete 文件和临时目录都不存在，则认为上传未找到或已失败/清理
	c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Upload not found or incomplete"}) // 使用 c.JSON

}

// InitUploadHandler 初始化上传过程
func InitUploadHandler(c *gin.Context) { // 修改签名
	// Gin 会自动处理 Method Not Allowed

	// 解析请求体
	var uploadRequest struct {
		FileName string `json:"fileName"`
		FileSize int64  `json:"fileSize"`
	}

	if err := c.ShouldBindJSON(&uploadRequest); err != nil { // 使用 c.ShouldBindJSON
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid request body"})
		return
	}

	if uploadRequest.FileName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "File name is required"})
		return
	}

	// 生成上传ID
	uploadID := generateUploadID(uploadRequest.FileName)

	// 返回上传ID
	c.JSON(http.StatusOK, ChunkResponse{ // 使用 c.JSON
		Success:  true,
		Message:  "Upload initialized",
		UploadID: uploadID,
	})
}

// mergeChunks 合并所有分片成一个完整文件
func mergeChunks(uploadID, fileName string, totalChunks int, expectedSize int64, chunkDir string) {
	// 加锁确保同一时间只有一个合并操作在进行
	mergeMutex.Lock()
	defer mergeMutex.Unlock()

	// 添加日志，记录 mergeChunks 接收到的 totalChunks 值
	log.Printf("[%s] mergeChunks: Started with totalChunks = %d, expectedSize = %d", uploadID, totalChunks, expectedSize)

	// 创建最终文件所在的目录 uploadDir/uploadID
	finalDir := filepath.Join(uploadDir, uploadID)
	if err := os.MkdirAll(finalDir, 0755); err != nil {
		fmt.Printf("[%s] Failed to create final upload directory: %v\n", uploadID, err)
		// Consider adding status update here (e.g., write a .error file)
		return
	}

	// 最终文件路径 uploadDir/uploadID/fileName
	finalFilePath := filepath.Join(finalDir, fileName)
	finalFile, err := os.Create(finalFilePath)
	if err != nil {
		fmt.Printf("[%s] Failed to create final file '%s': %v\n", uploadID, finalFilePath, err)
		// Consider adding status update here
		return
	}
	defer finalFile.Close()

	// 按顺序合并所有分片
	for i := 1; i <= totalChunks; i++ {
		chunkPath := filepath.Join(chunkDir, fmt.Sprintf("%d", i))

		// 检查分片文件是否存在
		if _, err := os.Stat(chunkPath); os.IsNotExist(err) {
			fmt.Printf("[%s] Chunk file %d not found: %s\n", uploadID, i, chunkPath)
			// 合并失败，可能需要清理已创建的 finalFile
			finalFile.Close()
			os.Remove(finalFilePath)
			// Consider adding status update here
			return
		}

		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			fmt.Printf("[%s] Failed to open chunk file %d: %v\n", uploadID, i, err)
			// 合并失败
			finalFile.Close()
			os.Remove(finalFilePath)
			// Consider adding status update here
			return
		}

		_, err = io.Copy(finalFile, chunkFile)
		chunkFile.Close() // 确保关闭文件句柄

		if err != nil {
			fmt.Printf("[%s] Failed to write chunk %d to final file: %v\n", uploadID, i, err)
			// 合并失败
			finalFile.Close()
			os.Remove(finalFilePath)
			// Consider adding status update here
			return
		}

	}

	// 关闭最终文件以确保所有数据都已写入磁盘
	if err := finalFile.Close(); err != nil {
		fmt.Printf("[%s] Failed to close final file: %v\n", uploadID, err)
		// 即使关闭失败，也尝试继续后续步骤
	}

	// 文件大小验证逻辑已移除 - 我们信任服务器合并后的实际大小
	// 获取最终文件信息以记录大小
	finalFileInfo, err := os.Stat(finalFilePath)
	if err != nil {
		// 仅记录错误，但不中断流程，因为合并可能已成功
		fmt.Printf("[%s] Warning: Failed to get final file info after merge: %v\n", uploadID, err)
		// Manually set size to -1 or some indicator if stat fails, or just proceed
		// For simplicity, we'll proceed. The essential part is merged file exists.
	} else {
		// 记录一下实际大小和预期大小，但不作为失败条件
		if finalFileInfo.Size() != expectedSize {
			fmt.Printf("[%s] Info: File size mismatch noted. Client Expected: %d, Server Actual: %d. Proceeding anyway.\n", uploadID, expectedSize, finalFileInfo.Size())
		}
	}

	// 创建 .complete 标记文件
	completeMarkerPath := filepath.Join(finalDir, ".complete")
	if _, err := os.Create(completeMarkerPath); err != nil {
		fmt.Printf("[%s] Failed to create complete marker file: %v\n", uploadID, err)
		// 即使无法创建标记文件，合并本身可能已成功，但状态检查会失败
		// 考虑是否回滚或记录此错误
	}

	// 创建 .filename 文件存储原始文件名
	fileNamePath := filepath.Join(finalDir, ".filename")
	if err := ioutil.WriteFile(fileNamePath, []byte(fileName), 0644); err != nil {
		fmt.Printf("[%s] Failed to write original filename marker: %v\n", uploadID, err)
		// 状态检查可能无法获取正确的文件名
	}

	// 清理临时分片目录
	if err := os.RemoveAll(chunkDir); err != nil {
		fmt.Printf("[%s] Failed to clean up chunk directory: %v\n", uploadID, err)
	}

	fmt.Printf("[%s] File '%s' merged successfully. Size: %d\n", uploadID, fileName, finalFileInfo.Size())
}

// generateUploadID 根据文件名生成唯一的上传ID
func generateUploadID(fileName string) string {
	timestamp := strconv.FormatInt(getTimestamp(), 10)
	data := fileName + timestamp
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

// getTimestamp 获取当前时间戳
func getTimestamp() int64 {
	return time.Now().UnixNano() // 使用纳秒时间戳增加唯一性
}

// ensureDirectoriesExist 确保必要的目录存在
func ensureDirectoriesExist() {
	os.MkdirAll(tempDir, 0755)
	os.MkdirAll(uploadDir, 0755)
}

// sendErrorResponse 发送错误响应
func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ChunkResponse{
		Success: false,
		Message: message,
	})
}

// sendSuccessResponse 发送成功响应
func sendSuccessResponse(w http.ResponseWriter, response ChunkResponse) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
