let isFileMode = false;
let maxFileSizeMB = 15; // Default, will be updated from config
const CHUNK_SIZE = 5 * 1024 * 1024; // 5MB chunk size
let serverConfig = {}; // To store config fetched from server
let quillInstance = null; // To store the Quill instance

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
const expirationSection = document.getElementById('expiration-section');
const expirationDurationInput = document.getElementById('expirationDuration'); // Renamed variable for clarity

// --- New Global Variables for Password Protection ---
let isPasswordProtected = false;
let passwordInput = null;

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

// Helper function to escape HTML characters
function escapeHTML(str) {
    const div = document.createElement('div');
    div.appendChild(document.createTextNode(str));
    return div.innerHTML;
}

// Removed old copyToClipboard and fallbackCopyToClipboard functions.
// Clipboard.js will handle the logic now.
// Function to guess MIME type based on filename extension
function getMimeType(filename) {
    const extension = filename.split('.').pop().toLowerCase();
    const mimeTypes = {
        // Common types
        'txt': 'text/plain',
        'md': 'text/markdown', // Added Markdown
        'html': 'text/html',
        'css': 'text/css',
        'js': 'application/javascript',
        'json': 'application/json',
        'xml': 'application/xml',
        'pdf': 'application/pdf',
        'zip': 'application/zip',
        'rar': 'application/vnd.rar',
        'tar': 'application/x-tar',
        'gz': 'application/gzip',
        '7z': 'application/x-7z-compressed',
        'jpg': 'image/jpeg',
        'jpeg': 'image/jpeg',
        'png': 'image/png',
        'gif': 'image/gif',
        'bmp': 'image/bmp',
        'svg': 'image/svg+xml',
        'webp': 'image/webp',
        'mp3': 'audio/mpeg',
        'wav': 'audio/wav',
        'ogg': 'audio/ogg',
        'mp4': 'video/mp4',
        'webm': 'video/webm',
        'avi': 'video/x-msvideo',
        'mov': 'video/quicktime',
        'doc': 'application/msword',
        'docx': 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
        'xls': 'application/vnd.ms-excel',
        'xlsx': 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
        'ppt': 'application/vnd.ms-powerpoint',
        'pptx': 'application/vnd.openxmlformats-officedocument.presentationml.presentation',
        // Add more as needed
    };
    return mimeTypes[extension] || null; // Return null if not found
   }

   // --- Config Fetching ---
   async function fetchAndApplyConfig() {
       try {
           const response = await fetch('/config');
           if (!response.ok) {
               console.error('Failed to fetch server config:', response.statusText);
               // Use default maxFileSizeMB if fetch fails
               return;
           }
           serverConfig = await response.json();
           console.log('Server config loaded:', serverConfig);

           // Apply Max File Size
           if (serverConfig.maxFileSizeMB) {
               maxFileSizeMB = serverConfig.maxFileSizeMB;
               console.log(`Max file size set to ${maxFileSizeMB} MB`);
           }

           // Apply Expiration settings
           if (serverConfig.expiration && serverConfig.expiration.enabled) {
               // Show the expiration input section if expiration is enabled on the server
               expirationSection.classList.remove('hidden');
               // Set placeholder based on server default if available
               if (serverConfig.expiration.default_duration) {
                   expirationDurationInput.placeholder = `例如: 30m, 1h, 7d (默认: ${serverConfig.expiration.default_duration})`;
               }
           } else {
               // Hide the section if expiration is disabled on the server
               expirationSection.classList.add('hidden');
           }

       } catch (error) {
           console.error('Error fetching or applying server config:', error);
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
    // 生成 12 字节（96位）的 IV，这是 AES-GCM 的推荐值
    const iv = window.crypto.getRandomValues(new Uint8Array(12));

    try {
        // 使用 AES-GCM 进行加密
        const encryptedContent = await window.crypto.subtle.encrypt(
            {
                name: "AES-GCM",
                iv: iv,
                tagLength: 128  // 显式设置认证标签长度为128位
            },
            encryptionKey,
            dataBuffer
        );

        // 验证加密结果
        if (!(encryptedContent instanceof ArrayBuffer)) {
            throw new Error("加密结果类型错误");
        }

        if (encryptedContent.byteLength < dataBuffer.byteLength) {
            throw new Error("加密数据大小异常");
        }

        return { encryptedContent, iv };
    } catch (error) {
        console.error("加密失败:", error);
        throw new Error("加密过程失败: " + error.message);
    }
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

    console.log("Form submitted. isFileMode:", isFileMode); // 新增日志，确认事件触发

    // 获取密码保护设置
    const enablePasswordCheckbox = document.getElementById('enablePassword');
    const accessPasswordInput = document.getElementById('accessPassword');
    const passwordNote = document.getElementById('passwordNote');
    const isPasswordProtected = enablePasswordCheckbox?.checked || false;
    const password = isPasswordProtected ? accessPasswordInput?.value : '';

    if (isPasswordProtected && (!password || password.length < 6)) {
        showStatus('访问密码至少需要6个字符', true);
        return; // 停止执行
    }

    if (passwordNote) {
        passwordNote.classList.toggle('hidden', !isPasswordProtected);
    }

    setLoading(true);

    try {
        const masterKeyBase64 = await generateMasterKey();
        const salt = window.crypto.getRandomValues(new Uint8Array(16)); // Salt for HKDF
        const encryptionKey = await deriveEncryptionKey(masterKeyBase64, salt);

        // 如果启用了密码保护，加密主密钥
        let encryptedMasterKey = null;
        if (isPasswordProtected) {
            const passwordSalt = window.crypto.getRandomValues(new Uint8Array(16));
            const passwordKey = await deriveKeyFromPassword(password, passwordSalt);
            const { encryptedContent, iv } = await encryptData(
                new TextEncoder().encode(masterKeyBase64),
                passwordKey
            );
            encryptedMasterKey = {
                data: arrayBufferToBase64(encryptedContent),
                iv: arrayBufferToBase64(iv),
                salt: arrayBufferToBase64(passwordSalt)
            };
        }

        // 根据模式调用相应的处理函数
        if (isFileMode) {
            console.log("Calling handleFileEncryption..."); // 新增日志
            await handleFileEncryption(masterKeyBase64, salt, encryptionKey, encryptedMasterKey);
        } else {
            console.log("Calling handleTextEncryption..."); // 新增日志
            await handleTextEncryption(masterKeyBase64, salt, encryptionKey, encryptedMasterKey);
        }

    } catch (error) {
        console.error("处理过程中出错:", error);
        showStatus('错误: ' + error.message, true);
        // 确保在任何错误情况下都重置加载状态
        setLoading(false);
    }
    // finally 块不再需要，因为 setLoading(false) 在成功路径（handleFile/TextEncryption内部）和错误路径（catch块）中都已处理。
    // 对于文件上传，setLoading(false) 在 finalizeUpload 成功或失败时调用。
    // 对于文本上传，setLoading(false) 在 handleTextEncryption 成功或失败时调用。
});

// --- Chunk Upload Functions ---
async function handleChunkUpload(originalFilename, originalFilesize, contentType, encryptedFileBuffer, iv, salt, masterKeyBase64, encryptedMasterKey) {
    showStatus("正在初始化分片上传...");

    // 1. Initialize Upload - 使用加密后的大小
    let uploadId;
    try {
        const initResponse = await fetch('/api/upload/init', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                fileName: originalFilename,
                fileSize: encryptedFileBuffer.byteLength  // 使用加密后的实际大小
            })
        });

        if (!initResponse.ok) {
            const errorData = await initResponse.json().catch(() => ({ message: '初始化上传失败' }));
            throw new Error('初始化失败 (' + initResponse.status + '): ' + errorData.message);
        }

        const initData = await initResponse.json();
        uploadId = initData.uploadId;
        if (!uploadId) {
            throw new Error("未能从服务器获取 Upload ID。");
        }
        console.log("Upload initialized with ID:", uploadId);
        console.log("Encrypted file size:", encryptedFileBuffer.byteLength);
    } catch (error) {
        showStatus('错误: ' + error.message, true);
        setLoading(false);
        return;
    }

    // 2. Upload Chunks
    const totalChunks = Math.ceil(encryptedFileBuffer.byteLength / CHUNK_SIZE);
    console.log('Starting chunk upload: ' + totalChunks + ' chunks');

    for (let chunkNumber = 1; chunkNumber <= totalChunks; chunkNumber++) {
        const start = (chunkNumber - 1) * CHUNK_SIZE;
        const end = Math.min(start + CHUNK_SIZE, encryptedFileBuffer.byteLength);
        const chunkBlob = new Blob([encryptedFileBuffer.slice(start, end)]);

        showStatus('正在上传分片 ' + chunkNumber + ' / ' + totalChunks + '...');

        const formData = new FormData();
        formData.append('uploadId', uploadId);
        formData.append('chunkNumber', chunkNumber.toString());
        formData.append('totalChunks', totalChunks.toString());
        formData.append('fileName', originalFilename); // Send original filename
        formData.append('fileSize', originalFilesize.toString()); // Send original filesize
        formData.append('chunk', chunkBlob, 'chunk_' + chunkNumber); // Add chunk blob

        try {
            const chunkResponse = await fetch('/api/upload/chunk', {
                method: 'POST',
                body: formData // Send as FormData
            });

            if (!chunkResponse.ok) {
                const errorData = await chunkResponse.json().catch(() => ({ message: '上传分片 ' + chunkNumber + ' 失败' }));
                throw new Error('上传分片 ' + chunkNumber + ' 失败 (' + chunkResponse.status + '): ' + errorData.message);
            }
            const chunkResult = await chunkResponse.json();
            console.log('Chunk ' + chunkNumber + ' uploaded:', chunkResult.message);

        } catch (error) {
            showStatus('错误: ' + error.message, true);
            setLoading(false);
            return; // Stop upload process
        }
    }

    // 3. Finalize Upload (Polling and Metadata Storage)
    showStatus("所有分片上传完毕，正在等待服务器合并...");
    await finalizeUpload(uploadId, iv, salt, originalFilename, contentType, encryptedFileBuffer.byteLength, masterKeyBase64, encryptedMasterKey); // Pass contentType and encrypted size
}

async function finalizeUpload(uploadId, iv, salt, originalFilename, contentType, fileSize, masterKeyBase64, encryptedMasterKey) {
    const pollInterval = 3000; // Poll every 3 seconds
    let attempts = 0;
    const maxAttempts = 20; // Max 1 minute of polling

    const poll = async () => {
        attempts++;
        if (attempts > maxAttempts) {
            throw new Error("服务器合并超时。请稍后再试。");
        }

        try {
            const statusResponse = await fetch('/api/upload/status?uploadId=' + uploadId);
            if (!statusResponse.ok) {
                // If status is 404, it might mean the merge failed and cleaned up, or still processing
                if (statusResponse.status === 404) {
                     console.log('Polling attempt ' + attempts + ': Upload status not found yet.');
                     setTimeout(poll, pollInterval); // Continue polling
                     return;
                }
                const errorData = await statusResponse.json().catch(() => ({ message: '检查状态失败' }));
                throw new Error('检查状态失败 (' + statusResponse.status + '): ' + errorData.message);
            }

            const statusData = await statusResponse.json();

            if (statusData.completed) {
                console.log("Server reported merge completed.");
                showStatus("文件合并成功，正在存储元数据...");

                // Store metadata using the new endpoint
                const ivBase64 = arrayBufferToBase64(iv);
                const saltBase64 = arrayBufferToBase64(salt);
                const metadataPayload = {
                	id: uploadId,
                	iv: ivBase64,
                	salt: saltBase64,
                	originalFilename: originalFilename,
                	contentType: contentType, // Include contentType
                	fileSize: fileSize,       // Include fileSize (encrypted size)
                	passwordProtection: encryptedMasterKey,
                	// Add setDuration if expiration section is visible and has a value
                	...(expirationSection && !expirationSection.classList.contains('hidden') && expirationDurationInput.value.trim() && { setDuration: expirationDurationInput.value.trim() })
                };

                const metaResponse = await fetch('/api/store/metadata', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(metadataPayload)
                });

                if (!metaResponse.ok) {
                    const errorData = await metaResponse.json().catch(() => ({ message: '存储元数据失败' }));
                    throw new Error('存储元数据失败 (' + metaResponse.status + '): ' + errorData.message);
                }

                const metaResult = await metaResponse.json();
                console.log("Metadata stored successfully:", metaResult);

                const shareUrl = window.location.origin +
                    window.location.pathname +
                    '?id=' + uploadId +
                    '#' + masterKeyBase64;

                showResult(shareUrl);
                setLoading(false); // Upload complete

            } else {
                // Still merging, poll again
                console.log('Polling attempt ' + attempts + ': Merge still in progress.');
                setTimeout(poll, pollInterval);
            }

        } catch (error) {
            showStatus('错误: ' + error.message, true);
            setLoading(false); // Stop loading on polling error
        }
    };

    // Start polling
    poll();
}

// --- Decryption on Load ---
async function handleDecryptionOnLoad() {
    const urlParams = new URLSearchParams(window.location.search);
    const dataId = urlParams.get('id');
    const masterKeyBase64 = window.location.hash.substring(1);

    if (!dataId || !masterKeyBase64) {
        return;
    }

    hideMessages();
    contentAreaDiv.classList.remove('hidden');
    decryptedContentDiv.innerHTML = '正在获取数据...';
    setLoading(true);

    try {
        console.log('Loading data for ID:', dataId);
        const response = await fetch(`/api/data/${dataId}`);

        if (response.status === 404) {
            throw new Error('数据不存在或已被销毁');
        }

        if (!response.ok) {
            throw new Error(`获取数据失败: ${response.statusText}`);
        }

        const responseData = await response.json();
        console.log('Server response received');

        // 如果数据需要密码保护
        if (responseData.passwordProtection) {
            // 如果还没有输入过密码
            if (!passwordInput) {
                console.log('Password protection detected, showing prompt');
                showPasswordPrompt(dataId);
                setLoading(false);
                return;
            }

            console.log('Verifying password...');
            try {
                // 验证密码
                const { data, iv, salt } = responseData.passwordProtection;
                if (!data || !iv || !salt) {
                    throw new Error('密码保护数据不完整');
                }

                console.log('Deriving password key...');
                const passwordKey = await deriveKeyFromPassword(
                    passwordInput,
                    base64ToArrayBuffer(salt)
                );

                console.log('Attempting to decrypt master key...');
                const decryptedMasterKeyBuffer = await decryptData(
                    base64ToArrayBuffer(data),
                    base64ToArrayBuffer(iv),
                    passwordKey
                );

                const decryptedMasterKey = new TextDecoder().decode(decryptedMasterKeyBuffer);
                console.log('Master key decrypted, verifying...');

                if (decryptedMasterKey !== masterKeyBase64) {
                    console.error('Master key verification failed');
                    throw new Error('密码错误');
                }
            } catch (error) {
                console.error('Password verification failed:', error);
                passwordInput = null; // 重置密码以便重试
                showPasswordPrompt(dataId);
                setLoading(false);
                if (error.message === '密码错误') {
                    showStatus('访问密码错误，请重试', true);
                } else {
                    showStatus('验证密码时出错: ' + error.message, true);
                }
                return;
            }
        }

        // 密码验证通过或不需要密码，继续处理数据
        console.log('Proceeding with data handling...');
        try {
        	// --- Robustly decode IV and Salt (needed for both file and text) ---
        	if (!responseData.iv || typeof responseData.iv !== 'string' || responseData.iv.length === 0) {
        		throw new Error("无效或缺失的 IV 数据");
        	}
        	if (!responseData.salt || typeof responseData.salt !== 'string' || responseData.salt.length === 0) {
        		throw new Error("无效或缺失的 Salt 数据");
        	}
        	let iv, salt;
        	try {
        		iv = base64ToArrayBuffer(responseData.iv);
        		salt = base64ToArrayBuffer(responseData.salt);
        	} catch (e) {
        		console.error("Failed to decode IV or Salt:", e);
        		throw new Error(`解码 IV 或 Salt 失败: ${e.message}. 数据可能已损坏或格式不正确。`);
        	}
        	// --- End robust decoding ---

        	const encryptionKey = await deriveEncryptionKey(masterKeyBase64, salt);

        	// 确定内容类型
        	const contentType = responseData.contentType || (responseData.originalFilename ? getMimeType(responseData.originalFilename) : 'text/plain'); // Guess MIME type for files

        	if (responseData.originalFilename) {
        		// --- 文件模式 ---
        		const originalFilename = responseData.originalFilename;
        		const fileId = dataId;
        		const fileIvBase64 = responseData.iv; // Store IV/Salt for later use
        		const fileSaltBase64 = responseData.salt;
        		console.log('文件模式检测到. 文件名:', originalFilename, '类型:', contentType);

        		// 检查是否支持预览
        		const canPreview = contentType && (
        			contentType.startsWith('image/') ||
        			contentType.startsWith('video/') ||
        			contentType.startsWith('audio/') ||
        			contentType === 'application/pdf'
        		);

        		decryptedContentDiv.innerHTML = `
        			<p>这是一个加密文件：<strong>${escapeHTML(originalFilename)}</strong></p>
        			${contentType ? `<p>文件类型: ${contentType}</p>` : ''}
        			<div id="file-preview-area" class="hidden" style="margin:15px 0; border:1px solid #ddd; padding:10px; max-width:100%; max-height:400px; overflow:auto;"></div>
        			<p id="file-status-msg">请选择操作方式：</p>
        			<div class="button-group" style="margin-top:10px;">
        				${canPreview ? '<button id="previewBtn" class="button secondary">预览文件</button>' : ''}
        				<button id="downloadBtn" class="button">下载文件</button>
        			</div>
        			<p><small>${canPreview ? '预览不会销毁文件。' : ''}下载后文件将从服务器删除。</small></p>
        		`;
        		const downloadBtn = document.getElementById('downloadBtn');
        		const previewBtn = document.getElementById('previewBtn');
        		const fileStatusMsg = document.getElementById('file-status-msg');
        		const previewArea = document.getElementById('file-preview-area');

        		// 添加预览按钮事件监听器
        		if (previewBtn) {
        			previewBtn.addEventListener('click', async () => {
        				// 调用之前添加的 handlePreview 函数
        				await handlePreview(
        					fileId,             // 文件ID (dataId)
        					masterKeyBase64,    // 主密钥
        					contentType,        // 文件类型
        					originalFilename,   // 原始文件名
        					fileIvBase64,       // IV (从 responseData 获取)
        					fileSaltBase64,     // Salt (从 responseData 获取)
        					previewArea,        // 预览区域元素
        					fileStatusMsg       // 状态消息元素
        				);
        			});
        		}

        		downloadBtn.onclick = async () => {
        			downloadBtn.disabled = true;
                    if (previewBtn) previewBtn.disabled = true; // Disable preview during download
        			downloadBtn.textContent = '处理中...'; // General processing text
        			fileStatusMsg.textContent = '正在下载加密文件...';
                    previewArea.classList.add('hidden'); // Hide preview area during download
                    previewArea.innerHTML = ''; // Clear preview area
        			setLoading(true); // Show loader during download/decrypt

        			try {
        				// 1. Fetch encrypted file data
        				console.log(`Fetching encrypted file from /api/download/${fileId}`);
        				const fetchResponse = await fetch(`/api/download/${fileId}`);
        				if (!fetchResponse.ok) {
        					// Try to get error message from server if possible
        					let errorMsg = `下载加密文件失败: ${fetchResponse.statusText}`;
        					try {
        						const errorData = await fetchResponse.json();
        						if (errorData.error) {
        							errorMsg = `下载加密文件失败 (${fetchResponse.status}): ${errorData.error}`;
        						}
        					} catch (e) { /* Ignore JSON parsing error */ }
        					throw new Error(errorMsg);
        				}
        				const encryptedFileBuffer = await fetchResponse.arrayBuffer();
        				console.log('Encrypted file downloaded, size:', encryptedFileBuffer.byteLength);
        				fileStatusMsg.textContent = '文件下载完毕，正在解密...';
        				downloadBtn.textContent = '正在解密...';

        				// 2. Prepare decryption parameters (already have responseData, iv, salt, masterKeyBase64 in scope)
        				// Re-decode IV and Salt robustly (already decoded above)
        				// let iv, salt; // Already defined and decoded
        				// try {
        				// 	if (!responseData.iv || typeof responseData.iv !== 'string' || responseData.iv.length === 0) throw new Error("无效或缺失的 IV 数据");
        				// 	if (!responseData.salt || typeof responseData.salt !== 'string' || responseData.salt.length === 0) throw new Error("无效或缺失的 Salt 数据");
        				// 	iv = base64ToArrayBuffer(responseData.iv);
        				// 	salt = base64ToArrayBuffer(responseData.salt);
        				// } catch (e) { throw new Error(`解码 IV/Salt 失败: ${e.message}`); }

        				// 3. Derive key (already derived above)
        				// const encryptionKey = await deriveEncryptionKey(masterKeyBase64, salt);

        				// 4. Validate sizes
        				if (iv.byteLength !== 12) throw new Error("无效的 IV 大小");
        				if (encryptedFileBuffer.byteLength < 16) throw new Error("加密数据过短"); // AES-GCM tag is 16 bytes (128 bits)

        				// 5. Decrypt data
        				console.log('Attempting decryption for download...');
        				const decryptedBuffer = await decryptData(encryptedFileBuffer, iv, encryptionKey);
        				console.log('File decrypted successfully for download, size:', decryptedBuffer.byteLength);
        				fileStatusMsg.textContent = '解密完成，准备下载...';

        				// 6. Create Blob
        				const blobMimeType = contentType || getMimeType(originalFilename) || 'application/octet-stream'; // Use determined contentType
        				const blob = new Blob([decryptedBuffer], { type: blobMimeType });

        				// 7. Create download link and trigger
        				const objectUrl = URL.createObjectURL(blob);
        				const a = document.createElement('a');
        				a.href = objectUrl;
        				a.download = originalFilename; // Use the original filename
        				document.body.appendChild(a); // Required for Firefox
        				a.click();
        				document.body.removeChild(a);
        				console.log('Download triggered for:', originalFilename);
        				fileStatusMsg.textContent = '下载已开始！';
        				downloadBtn.textContent = '下载完成'; // Or hide it

        				// 8. Revoke Object URL
        				URL.revokeObjectURL(objectUrl);

        				// 9. Send burn request *after* successful decryption and download trigger
        				console.log('Sending burn request for file metadata after successful processing...');
        				try {
        					await fetch(`/api/burn/${fileId}`, { method: 'POST' });
        					console.log('Burn request sent successfully for file metadata.');
        					fileStatusMsg.innerHTML += '<br><small>服务器记录已删除。</small>';
        				} catch (burnError) {
        					console.error('Burn request failed for file:', burnError);
        					fileStatusMsg.innerHTML += '<br><small style="color: orange;">警告：无法从服务器删除此文件记录。</small>';
        				}

        			} catch (error) {
        				console.error("File download/decryption failed:", error);
        				fileStatusMsg.innerHTML = `<span style="color: red;">处理文件时出错: ${error.message}</span>`;
        				downloadBtn.textContent = '处理失败';
        				downloadBtn.disabled = false; // Re-enable button on error
                        if (previewBtn) previewBtn.disabled = false; // Re-enable preview button on error
        			} finally {
        				setLoading(false); // Hide loader
        			}
        		};
        		setLoading(false); // Initial loading state after getting metadata

        	} else {
        		// --- Text Mode ---
        		console.log('Text mode detected.');
        		// --- Robustly check and decode EncryptedData ---
        		const encryptedData = responseData.encryptedData;
        		if (!encryptedData || typeof encryptedData !== 'string' || encryptedData.length === 0) {
        			console.error("Encrypted data missing or invalid for text mode:", encryptedData);
        			throw new Error("未找到或无效的加密文本数据。");
        		}
        		decryptedContentDiv.innerHTML = '正在解密文本...';
        		console.log('Attempting to decrypt text data');
        		let encryptedBuffer;
        		 try {
        			encryptedBuffer = base64ToArrayBuffer(encryptedData);
        		} catch (e) {
        			console.error("Failed to decode encryptedData:", e);
        			throw new Error(`解码加密文本失败: ${e.message}. 数据可能已损坏或格式不正确。`);
        		}
        		// --- End robust check ---

        		// Validate IV size (important for AES-GCM)
        		if (iv.byteLength !== 12) {
        			throw new Error("无效的加密参数 (IV size)");
        		}
        		// Validate encrypted data size (must be at least tag length)
        		if (encryptedBuffer.byteLength < 16) { // 128-bit tag = 16 bytes
        			throw new Error("加密数据无效或已损坏 (too short)");
        		}

        		try {
        			const decryptedBuffer = await decryptData(encryptedBuffer, iv, encryptionKey);
        			const decryptedText = new TextDecoder().decode(decryptedBuffer);

                    // --- Render HTML directly (assuming Quill provides HTML) ---
                    console.log('Rendering content as HTML...');
                    // Sanitize potentially harmful HTML before rendering
                    // Allow pre and code tags, and common attributes like class for highlighting
                    const cleanHtml = DOMPurify.sanitize(decryptedText, {
                        USE_PROFILES: { html: true }, // Use default HTML profile
                        ADD_TAGS: ['pre'], // Ensure pre is allowed if not default
                        ADD_ATTR: ['class'] // Allow class attribute for hljs
                    });
                    decryptedContentDiv.innerHTML = cleanHtml;

                    // Apply syntax highlighting to Quill's code blocks
                    // Quill typically uses <pre class="ql-syntax" spellcheck="false">...</pre>
                    decryptedContentDiv.querySelectorAll('pre.ql-syntax, pre code').forEach((block) => { // Target Quill's class and standard pre>code
                        // If it's a pre element directly, highlight it. If it's a code inside pre, highlight code.
                        const targetElement = block.tagName === 'PRE' ? block : block;
                        console.log('Highlighting block:', targetElement);
                        try {
                            hljs.highlightElement(targetElement);
                        } catch(e) {
                            console.error("Highlight.js error:", e, "on element:", targetElement);
                        }

                        // Add copy button to the <pre> element
                        const preElement = block.tagName === 'PRE' ? block : block.parentNode;
                        if (preElement.tagName !== 'PRE' || preElement.querySelector('.copy-code-button')) {
                             // Skip if not a PRE or if button already exists (e.g., nested code)
                             return;
                        }

                        const copyButton = document.createElement('button');
                        copyButton.textContent = '复制';
                        copyButton.className = 'copy-code-button button secondary small';
                        copyButton.style.position = 'absolute';
                        copyButton.style.top = '5px';
                        copyButton.style.right = '5px';
                        copyButton.style.opacity = '0.7';
                        copyButton.style.zIndex = '1'; // Ensure button is clickable

                        // Get text content directly from the <pre> element for the data attribute
                        const codeToCopy = preElement.textContent || '';
                        if (codeToCopy) {
                             copyButton.setAttribute('data-clipboard-text', codeToCopy);
                             console.log('Set data-clipboard-text for button.');
                        } else {
                             console.error('Could not extract code to set data-clipboard-text.');
                             copyButton.disabled = true; // Disable button if no text
                             copyButton.textContent = '错误';
                        }

                        // Remove the old onclick handler assignment
                        // copyButton.onclick = ... (Removed)

                        preElement.style.position = 'relative'; // Needed for absolute positioning
                        preElement.appendChild(copyButton); // Append button to pre
                    });
                    // --- End rendering logic ---

                    setLoading(false);

        			// Send burn request for text data
        			console.log('Sending burn request for text...');
        			try {
        				await fetch(`/api/burn/${dataId}`, { method: 'POST' });
        				decryptedContentDiv.innerHTML += '<br><br><small>此消息已从服务器删除。</small>';
        			} catch (error) {
        				console.error('Burn request failed for text:', error);
        				decryptedContentDiv.innerHTML += '<br><br><small style="color: orange;">警告：无法从服务器删除此消息。</small>';
        			}
        		} catch (decryptError) {
        			console.error("Decryption failed:", decryptError);
        			throw new Error("解密失败：密钥可能无效或数据已损坏");
        		}
        	}
        } catch (error) {
        	console.error("Data processing failed:", error);
        	// Display the error message caught within this block
        	throw new Error(error.message || "数据处理失败");
        }
    } catch (error) {
        console.error("Decryption process failed:", error);
        decryptedContentDiv.innerHTML = `<span style="color: red;">错误: ${error.message}</span>`;
        setLoading(false);
    }
}

// --- Password Protection Functions ---
function showPasswordPrompt(dataId) {
    const promptDiv = document.createElement('div');
    promptDiv.id = 'password-prompt';
    promptDiv.innerHTML = `
        <h3>需要密码</h3>
        <p>此内容受密码保护。请输入密码以继续：</p>
        <input type="password" id="password-input-field" placeholder="输入访问密码" required>
        <button id="password-submit-btn" class="button">提交</button>
        <p id="password-error" class="error-message hidden"></p>
    `;
    decryptedContentDiv.innerHTML = ''; // Clear previous content
    decryptedContentDiv.appendChild(promptDiv);

    const inputField = document.getElementById('password-input-field');
    const submitButton = document.getElementById('password-submit-btn');
    const errorP = document.getElementById('password-error');

    submitButton.onclick = () => {
        const enteredPassword = inputField.value;
        if (!enteredPassword) {
            errorP.textContent = '请输入密码';
            errorP.classList.remove('hidden');
            return;
        }
        passwordInput = enteredPassword; // Store password globally for this attempt
        handleDecryptionOnLoad(); // Re-run decryption logic with the password
    };

    inputField.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            submitButton.click();
        }
    });
}

async function submitPassword(id) {
    const passwordField = document.getElementById('password-input-field');
    const password = passwordField.value;
    if (!password) {
        document.getElementById('password-error').textContent = '请输入密码';
        document.getElementById('password-error').classList.remove('hidden');
        return;
    }
    passwordInput = password; // Store password globally
    handleDecryptionOnLoad(); // Re-run decryption logic
}

// Derive key from password using PBKDF2
async function deriveKeyFromPassword(password, salt) {
    const encoder = new TextEncoder();
    const passwordBuffer = encoder.encode(password);

    // Import the password material into a CryptoKey
    const baseKey = await window.crypto.subtle.importKey(
        "raw",
        passwordBuffer,
        { name: "PBKDF2" },
        false, // Not extractable
        ["deriveKey"]
    );

    // Derive the key using PBKDF2
    const derivedKey = await window.crypto.subtle.deriveKey(
        {
            name: "PBKDF2",
            salt: salt,
            iterations: 100000, // Recommended minimum iterations
            hash: "SHA-256"
        },
        baseKey,
        { name: "AES-GCM", length: 256 }, // Key type for AES-GCM
        true, // Allow export (needed for decryption)
        ["encrypt", "decrypt"] // Key usages
    );

    return derivedKey;
}

// Function to initialize password protection checkbox logic
function initializePasswordProtection() {
    console.log("Initializing password protection logic..."); // Log start
    const enablePasswordCheckbox = document.getElementById('enablePassword');
    const passwordInputGroup = document.getElementById('passwordInputGroup');
    const accessPasswordInput = document.getElementById('accessPassword');
    const passwordNote = document.getElementById('passwordNote');

    // Log whether each element was found
    console.log("enablePasswordCheckbox found:", !!enablePasswordCheckbox);
    console.log("passwordInputGroup found:", !!passwordInputGroup);
    console.log("accessPasswordInput found:", !!accessPasswordInput);
    console.log("passwordNote found:", !!passwordNote);

    if (enablePasswordCheckbox && passwordInputGroup && accessPasswordInput && passwordNote) {
        console.log("All password elements found. Adding event listener."); // Log if condition passes
        enablePasswordCheckbox.addEventListener('change', () => {
            console.log("Password checkbox 'change' event triggered."); // Log event trigger
            const isChecked = enablePasswordCheckbox.checked;
            console.log("Checkbox is checked:", isChecked); // Log checkbox state
            passwordInputGroup.classList.toggle('hidden', !isChecked);
            passwordNote.classList.toggle('hidden', !isChecked);
            console.log("Toggled 'hidden' class on passwordInputGroup and passwordNote."); // Log class toggle
            if (!isChecked) {
                accessPasswordInput.value = ''; // Clear password if disabled
            }
        });
    } else {
        console.warn("One or more password protection elements not found in the DOM. Event listener NOT added."); // More specific warning
    }
}
// --- Encryption and Upload Logic ---

// Handle File Encryption and Start Upload
async function handleFileEncryption(masterKeyBase64, salt, encryptionKey, encryptedMasterKey) {
    const file = fileInput.files[0];
    if (!file) {
        showStatus('请选择一个文件', true);
        setLoading(false);
        return;
    }

    showStatus('正在读取文件...');
    const reader = new FileReader();

    reader.onload = async (e) => {
        try {
            showStatus('正在加密文件...');
            const fileBuffer = e.target.result;
            const { encryptedContent, iv } = await encryptData(fileBuffer, encryptionKey);

            showStatus('文件加密完成，准备上传...');
            // Determine content type
            const contentType = getMimeType(file.name) || 'application/octet-stream'; // Default if unknown
            console.log(`Determined Content-Type: ${contentType}`);
         
            // Pass original filename, size, contentType along with encrypted data
            await handleChunkUpload(file.name, file.size, contentType, encryptedContent, iv, salt, masterKeyBase64, encryptedMasterKey);
            // setLoading(false) is handled within handleChunkUpload/finalizeUpload

        } catch (error) {
            console.error("文件加密或上传准备失败:", error);
            showStatus('错误: ' + error.message, true);
            setLoading(false); // Ensure loading is stopped on error
        }
    };

    reader.onerror = () => {
        showStatus('读取文件失败', true);
        setLoading(false);
    };

    reader.readAsArrayBuffer(file);
}

// Handle Text Encryption and Upload
async function handleTextEncryption(masterKeyBase64, salt, encryptionKey, encryptedMasterKey) {
    // Get content from Quill instance (as HTML)
    const messageHtml = quillInstance ? quillInstance.root.innerHTML : '';
    // Basic check if editor is empty (Quill might insert <p><br></p> for empty)
    const isEmpty = !quillInstance || quillInstance.getLength() <= 1;
    const message = isEmpty ? '' : messageHtml; // Use HTML content
    if (!message) {
        showStatus('请输入文本内容', true);
        setLoading(false);
        return;
    }

    showStatus('正在加密文本...');
    try {
        const messageBuffer = new TextEncoder().encode(message);
        const { encryptedContent, iv } = await encryptData(messageBuffer, encryptionKey);

        showStatus('加密完成，正在保存...');

        const ivBase64 = arrayBufferToBase64(iv);
        const saltBase64 = arrayBufferToBase64(salt);
        const encryptedDataBase64 = arrayBufferToBase64(encryptedContent);

        const textFormatSelect = document.getElementById('textFormat'); // Get the select element
        const selectedContentType = textFormatSelect ? textFormatSelect.value : 'text/plain'; // Get selected value or default
      
        const payload = {
        	encryptedData: encryptedDataBase64,
        	iv: ivBase64,
        	salt: saltBase64,
        	contentType: selectedContentType, // Add the selected content type
        	passwordProtection: encryptedMasterKey,
        	// Add setDuration if expiration section is visible and has a value
        	...(expirationSection && !expirationSection.classList.contains('hidden') && expirationDurationInput.value.trim() && { setDuration: expirationDurationInput.value.trim() })
        };

        const response = await fetch('/api/store', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });

        if (!response.ok) {
            const errorData = await response.json()
                .catch(() => ({ error: '无法解析服务器错误响应' }));
            throw new Error('服务器错误 (' + response.status + '): ' +
                (errorData.error || response.statusText));
        }

        const resultData = await response.json();
        if (!resultData.id) {
            throw new Error("服务器未返回数据 ID");
        }

        const shareUrl = window.location.origin +
            window.location.pathname +
            '?id=' + resultData.id +
            '#' + masterKeyBase64;

        showResult(shareUrl);
        setLoading(false);
    } catch (error) {
        console.error("文本加密过程失败:", error);
        showError("加密或保存失败: " + error.message);
        setLoading(false);
    }
}
// 文件预览处理函数
async function handlePreview(fileId, masterKeyBase64, contentType, filename, ivBase64, saltBase64, previewArea, statusMsg) {
    try {
        previewArea.innerHTML = '正在准备预览...';
        previewArea.classList.remove('hidden');
        setLoading(true);

        // 获取加密数据
        console.log(`Fetching preview data for ${fileId}`);
        const response = await fetch(`/api/download/${fileId}`, {
            headers: { 'Accept': contentType }
        });
        if (!response.ok) {
            throw new Error(`获取预览数据失败: ${response.statusText}`);
        }
        const encryptedData = await response.arrayBuffer();

        // 解密数据
        const iv = base64ToArrayBuffer(ivBase64);
        const salt = base64ToArrayBuffer(saltBase64);
        const encryptionKey = await deriveEncryptionKey(masterKeyBase64, salt);
        const decryptedData = await decryptData(encryptedData, iv, encryptionKey);

        // 根据文件类型创建预览
        if (contentType.startsWith('image/')) {
            const blob = new Blob([decryptedData], { type: contentType });
            const url = URL.createObjectURL(blob);
            previewArea.innerHTML = `<img src="${url}" style="max-width:100%; max-height:400px;">`;
        } else if (contentType.startsWith('video/')) {
            const blob = new Blob([decryptedData], { type: contentType });
            const url = URL.createObjectURL(blob);
            previewArea.innerHTML = `
                <video controls style="max-width:100%; max-height:400px;">
                    <source src="${url}" type="${contentType}">
                    您的浏览器不支持视频预览
                </video>
            `;
        } else if (contentType.startsWith('audio/')) {
            const blob = new Blob([decryptedData], { type: contentType });
            const url = URL.createObjectURL(blob);
            previewArea.innerHTML = `
                <audio controls style="width:100%">
                    <source src="${url}" type="${contentType}">
                    您的浏览器不支持音频预览
                </audio>
            `;
        } else if (contentType === 'application/pdf') {
            const blob = new Blob([decryptedData], { type: contentType });
            const url = URL.createObjectURL(blob);
            previewArea.innerHTML = `<iframe src="${url}" style="width:100%; height:400px; border:none;"></iframe>`;
        } else {
            previewArea.innerHTML = '不支持预览此文件类型';
        }

        statusMsg.textContent = '预览加载完成';
    } catch (error) {
        console.error('预览失败:', error);
        previewArea.innerHTML = `<span style="color:red">预览失败: ${error.message}</span>`;
    } finally {
        setLoading(false);
    }
}

// --- Initialization ---
document.addEventListener('DOMContentLoaded', () => {
    fetchAndApplyConfig(); // Fetch config on page load
    handleDecryptionOnLoad(); // Check if URL has ID and key
    initializePasswordProtection(); // Setup password checkbox listener

    // Initialize Quill on the editor container
    const editorContainer = document.getElementById('editor-container');
    if (editorContainer) {
        console.log("Found #editor-container div. Attempting to initialize Quill...");
        try {
            // Define Quill toolbar options
            const toolbarOptions = [
                [{ 'header': [1, 2, 3, false] }],
                ['bold', 'italic', 'underline', 'strike'],        // toggled buttons
                ['blockquote', 'code-block'],

                [{ 'list': 'ordered'}, { 'list': 'bullet' }],
                [{ 'script': 'sub'}, { 'script': 'super' }],      // superscript/subscript
                [{ 'indent': '-1'}, { 'indent': '+1' }],          // outdent/indent

                [{ 'color': [] }, { 'background': [] }],          // dropdown with defaults from theme
                [{ 'font': [] }],
                [{ 'align': [] }],

                ['link', 'image'], // Link and image buttons

                ['clean']                                         // remove formatting button
            ];

            quillInstance = new Quill('#editor-container', {
                modules: {
                    toolbar: toolbarOptions,
                     syntax: true // Enable syntax highlighting module if using quilljs-syntax
                },
                theme: 'snow', // Use 'snow' theme (includes toolbar)
                placeholder: '在此输入你的秘密消息...'
            }); // End of new Quill() options
            console.log("Quill initialized successfully."); // More specific log

             // Check for core Quill elements and log dimensions after a short delay
             setTimeout(() => {
                 // Quill usually inserts the toolbar *before* the editor container div
                 const quillToolbar = editorContainer.previousElementSibling; // Check previous sibling
                 const quillEditorArea = editorContainer.querySelector('.ql-editor'); // Editor area is inside

                 if (quillToolbar && quillToolbar.classList.contains('ql-toolbar')) {
                     console.log("Quill toolbar (.ql-toolbar) found (as previous sibling). OffsetHeight:", quillToolbar.offsetHeight, "OffsetWidth:", quillToolbar.offsetWidth);
                 } else {
                      // Fallback: Check parent's children just in case structure differs slightly
                      const parentToolbar = editorContainer.parentNode.querySelector('.ql-toolbar');
                      if(parentToolbar) {
                           console.log("Quill toolbar (.ql-toolbar) found (within parent). OffsetHeight:", parentToolbar.offsetHeight, "OffsetWidth:", parentToolbar.offsetWidth);
                      } else {
                           console.error("Quill toolbar (.ql-toolbar) NOT found near #editor-container.");
                      }
                 }

                 if (quillEditorArea) {
                     console.log("Quill editor area (.ql-editor) found. OffsetHeight:", quillEditorArea.offsetHeight, "OffsetWidth:", quillEditorArea.offsetWidth);
                     console.log("Quill editor area innerHTML (sample):", quillEditorArea.innerHTML.substring(0, 100));
                 } else {
                     console.error("Quill editor area (.ql-editor) NOT found inside #editor-container.");
                 }
             }, 100); // 100ms delay

        } catch (error) {
            console.error("Failed to initialize Quill:", error);
        }
    } else {
        console.error("Div element with ID 'editor-container' not found for Quill initialization.");
    }

    // Initialize ClipboardJS *after* other initializations within DOMContentLoaded
    try {
        const clipboard = new ClipboardJS('.copy-code-button');

        clipboard.on('success', function(e) {
            console.info('ClipboardJS success:', e);
            const button = e.trigger;
            const originalText = button.textContent;
            button.textContent = '已复制!';
            button.disabled = true;
             setTimeout(() => {
                 button.textContent = originalText;
                 button.disabled = false;
             }, 1500);
            e.clearSelection();
            // Initialize ClipboardJS *after* other initializations within DOMContentLoaded
            try {
                const clipboard = new ClipboardJS('.copy-code-button');
        
                clipboard.on('success', function(e) {
                    console.info('ClipboardJS success:', e);
                    const button = e.trigger;
                    const originalText = button.textContent;
                    button.textContent = '已复制!';
                    button.disabled = true;
                     setTimeout(() => {
                         button.textContent = originalText;
                         button.disabled = false;
                     }, 1500);
                    e.clearSelection();
                });
        
                clipboard.on('error', function(e) {
                    console.error('ClipboardJS error:', e);
                     const button = e.trigger;
                     const originalText = button.textContent;
                     button.textContent = '失败!';
                     button.disabled = true;
                     setTimeout(() => {
                         button.textContent = originalText;
                         button.disabled = false;
                     }, 2000);
                    showStatus('复制失败，请手动复制。', true);
                });
        
                console.log("ClipboardJS initialized for .copy-code-button elements.");
            } catch(error) {
                 console.error("Failed to initialize ClipboardJS:", error);
            }
        }); // End of DOMContentLoaded listener

        clipboard.on('error', function(e) {
            console.error('ClipboardJS error:', e);
             const button = e.trigger;
             const originalText = button.textContent;
             button.textContent = '失败!';
             button.disabled = true;
             setTimeout(() => {
                 button.textContent = originalText;
                 button.disabled = false;
             }, 2000);
            showStatus('复制失败，请手动复制。', true);
        });

        console.log("ClipboardJS initialized for .copy-code-button elements.");
    } catch(error) {
         console.error("Failed to initialize ClipboardJS:", error);
    }
}); // End of DOMContentLoaded listener
