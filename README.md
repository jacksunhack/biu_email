# Nyaa~crypted Kitty Note (biu_email)

这是一个简单的Web应用程序，用于创建加密的、阅后即焚的文本笔记和文件。用户可以输入文本消息或上传文件，应用程序会对其进行加密，并生成一个唯一的短链接。当有人通过该短链接访问时，消息或文件将被解密显示，然后立即从服务器删除。

## ✨ 功能特性

*   **加密文本笔记:** 使用OpenPGP加密文本消息。
*   **加密文件上传:** 使用AES-GCM加密上传的文件（前端限制最大15MB）。
*   **阅后即焚:** 消息和文件在首次通过链接访问后会自动从服务器删除。
*   **短链接生成:** 为每个加密的消息或文件生成易于分享的短链接。
*   **Web界面:** 提供一个简单的前端界面进行操作。
*   **Docker化部署:** 使用Docker和Docker Compose轻松部署。
*   **可配置:** 通过`config.yaml`文件进行基本配置。

## 🛠️ 技术栈

*   **后端:** Go, Gin Web Framework
*   **前端:** HTML, CSS, JavaScript, OpenPGP.js, Web Crypto API
*   **部署:** Docker, Docker Compose
*   **配置:** YAML

## 🚀 快速开始

本项目设计为使用Docker运行。

**先决条件:**

*   Docker ([https://www.docker.com/get-started](https://www.docker.com/get-started))
*   Docker Compose ([https://docs.docker.com/compose/install/](https://docs.docker.com/compose/install/))

**安装与运行:**

1.  **克隆仓库:**
    ```bash
    git clone https://github.com/jacksunhack/biu_email.git
    cd biu_email
    ```

2.  **创建数据目录:**
    应用程序需要持久化存储目录。在`biu_email`目录下创建这些目录并设置适当的权限（如果您的Docker用户不是root，可能需要调整权限）：
    ```bash
    mkdir -p biu_email_data/{messages,temp-files,logs,storage}
    # 在Linux/macOS上，确保Docker运行用户有写入权限
    chmod -R 777 biu_email_data
    ```
    *注意: `chmod 777` 是为了方便演示，生产环境请根据安全需要设置更严格的权限。*

3.  **配置 (可选):**
    检查 `config.yaml` 文件。默认配置监听 `0.0.0.0:3003`。

4.  **构建并启动容器:**
    ```bash
    docker-compose up --build -d
    ```
    *   `--build` 会强制重新构建镜像。
    *   `-d` 会在后台运行容器。

5.  **访问应用:**
    在浏览器中打开 `http://<your-server-ip>:3003` (或者 `http://localhost:3003` 如果在本地运行)。

**停止服务:**

```bash
docker-compose down
```

## ⚙️ 配置

主要的配置在 `config.yaml` 文件中：

*   `application.name`: 应用名称。
*   `server.host`: 服务器监听的主机地址 (在Docker容器内通常是 `0.0.0.0`)。
*   `server.port`: 服务器监听的端口。
*   `logging`: 日志级别和处理程序配置。

## 📝 使用说明

1.  打开Web界面。
2.  选择创建加密消息或上传加密文件。
3.  输入消息或选择文件。
4.  点击 "ENCRYPT_AND PURR-TRANSMIT" 按钮。
5.  系统将生成一个阅后即焚的短链接。
6.  将此链接分享给接收者。
7.  接收者打开链接后，消息或文件将显示，并从服务器删除。再次访问链接将提示消息已被销毁。

## 🤝 贡献

欢迎提交Pull Request或提出Issue。

## 📄 许可证

本项目采用 MIT 许可证。详情请参阅 [LICENSE](LICENSE) 文件。
