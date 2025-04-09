# Biu~ 阅后即焚 (客户端加密版)

这是一个安全的Web应用程序，所有加密操作都在客户端完成，支持加密文本和文件分享，阅后即焚。

## ✨ 功能特性

* **安全加密**:
  - 文本/文件: AES-GCM 256位加密
  - 客户端密钥生成，服务器无法解密
* **文件支持**:
  - 分片上传大文件(可配置大小)
  - 断点续传，5MB分片大小
* **数据管理**:
  - 阅后即焚(首次访问后自动删除)
  - 手动销毁功能
* **部署特性**:
  - 单容器Docker部署
  - 最小化依赖

## 🛠️ 技术栈

* **后端**: Go 1.21+, Gin框架
* **前端**: Web Crypto API, Vanilla JS
* **存储**: 本地文件系统
* **部署**: Docker + Docker Compose

## 🚀 快速开始

```bash
# 1. 克隆仓库
git clone https://github.com/jacksunhack/biu_email.git
cd biu_email

# 2. 准备存储目录
mkdir -p storage/{data,temp_uploads,uploads}

# 3. 启动服务
docker-compose up -d --build
```

## ⚙️ 配置

修改config.yaml:
```yaml
server:
  max_file_size_mb: 100  # 最大文件上传大小(MB)
  port: 3003
```

## ✨ 未来展望

### 已实现功能
✓ 分片上传大文件支持  
✓ 客户端流式加密/解密  
✓ 元数据与文件分离存储  

### 计划功能
◉ 链接访问密码保护  
◉ 自定义有效期(1小时/1天/1周)  
◉ 下载次数限制  
◉ 管理后台(查看/清理文件)  
◉ Prometheus监控集成  

## 📄 许可证
MIT License