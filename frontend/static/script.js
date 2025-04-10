let isFileMode = false;
let maxFileSizeMB = 15; // Default, will be updated from config
const CHUNK_SIZE = 5 * 1024 * 1024; // 5MB chunk size

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


// Function to guess MIME type based on filename extension
function getMimeType(filename) {
    const extension = filename.split('.').pop().toLowerCase();
    const mimeTypes = {
        // Common types
        'txt': 'text/plain',
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
async function handleChunkUpload(originalFilename, originalFilesize, encryptedFileBuffer, iv, salt, masterKeyBase64, encryptedMasterKey) {
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
    await finalizeUpload(uploadId, iv, salt, originalFilename, masterKeyBase64, encryptedMasterKey);
}

async function finalizeUpload(uploadId, iv, salt, originalFilename, masterKeyBase64, encryptedMasterKey) {
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
                    passwordProtection: encryptedMasterKey
                };

                const metaResponse = await fetch('/api/store/metadata', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(metadataPayload)
                });

                if (!metaResponse.ok) {
                    const errorData = await metaResponse.json().catch(() => ({ error: '存储元数据失败' }));
                    throw new Error('存储元数据失败 (' + metaResponse.status + '): ' + errorData.error);
                }

                console.log("Metadata stored successfully for ID:", uploadId);
                const shareUrl = window.location.origin + window.location.pathname + '?id=' + uploadId + '#' + masterKeyBase64;
                showResult(shareUrl);
                setLoading(false); // Final success

            } else {
                // Not completed yet, poll again
                console.log('Polling attempt ' + attempts + ': Merge not complete yet.');
                showStatus('正在等待服务器合并... (' + attempts + '/' + maxAttempts + ')');
                setTimeout(poll, pollInterval);
            }
        } catch (error) {
            showStatus('错误: ' + error.message, true);
            setLoading(false);
        }
    };

    setTimeout(poll, pollInterval); // Start polling
}

// --- Decryption Logic (on page load if URL contains ID and Key) ---
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
      
        	if (responseData.originalFilename) {
        		// --- File Mode ---
        		const originalFilename = responseData.originalFilename; // Store filename
        		console.log('File mode detected. Filename:', originalFilename);
        		decryptedContentDiv.innerHTML = `
        			<p>这是一个加密文件：<strong>${escapeHTML(originalFilename)}</strong></p>
        			<p id="file-status-msg">点击下方按钮开始下载并解密文件。</p>
        			<button id="downloadBtn" class="button">下载并解密文件</button>
        			<p><small>点击下载后，文件将从服务器永久删除。</small></p>
        		`;
        		const downloadBtn = document.getElementById('downloadBtn');
        		const fileStatusMsg = document.getElementById('file-status-msg');
     
        		downloadBtn.onclick = async () => {
        			downloadBtn.disabled = true;
        			downloadBtn.textContent = '处理中...'; // General processing text
        			fileStatusMsg.textContent = '正在下载加密文件...';
        			setLoading(true); // Show loader during download/decrypt
     
        			try {
        				// 1. Fetch encrypted file data
        				console.log(`Fetching encrypted file from /api/download/${dataId}`);
        				const fetchResponse = await fetch(`/api/download/${dataId}`);
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
        				// Re-decode IV and Salt robustly
        				let iv, salt;
        				try {
        					if (!responseData.iv || typeof responseData.iv !== 'string' || responseData.iv.length === 0) throw new Error("无效或缺失的 IV 数据");
        					if (!responseData.salt || typeof responseData.salt !== 'string' || responseData.salt.length === 0) throw new Error("无效或缺失的 Salt 数据");
        					iv = base64ToArrayBuffer(responseData.iv);
        					salt = base64ToArrayBuffer(responseData.salt);
        				} catch (e) { throw new Error(`解码 IV/Salt 失败: ${e.message}`); }
     
        				// 3. Derive key
        				const encryptionKey = await deriveEncryptionKey(masterKeyBase64, salt);
     
        				// 4. Validate sizes
        				if (iv.byteLength !== 12) throw new Error("无效的 IV 大小");
        				if (encryptedFileBuffer.byteLength < 16) throw new Error("加密数据过短"); // AES-GCM tag is 16 bytes (128 bits)
     
        				// 5. Decrypt data
        				console.log('Attempting decryption...');
        				const decryptedBuffer = await decryptData(encryptedFileBuffer, iv, encryptionKey);
        				console.log('File decrypted successfully, size:', decryptedBuffer.byteLength);
        				fileStatusMsg.textContent = '解密完成，准备下载...';
     
        				// 6. Create Blob
        				const mimeType = getMimeType(originalFilename) || 'application/octet-stream';
        				const blob = new Blob([decryptedBuffer], { type: mimeType });
     
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
        					await fetch(`/api/burn/${dataId}`, { method: 'POST' });
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
        			// Use textContent for security against XSS if the text might contain HTML
        			decryptedContentDiv.textContent = decryptedText;
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

function showPasswordPrompt(dataId) {
    decryptedContentDiv.innerHTML = `
        <div class="password-prompt">
            <h3>此内容受密码保护</h3>
            <div class="form-group">
                <input type="password" id="contentPassword" placeholder="请输入访问密码" class="form-control">
                <button onclick="submitPassword('${dataId}')" class="btn btn-primary">解锁内容</button>
            </div>
        </div>
    `;
}

// 更新提交密码的函数
async function submitPassword(id) {
    const input = document.getElementById('contentPassword');
    if (!input || !input.value) {
        showStatus('请输入密码', true);
        return;
    }

    try {
        setLoading(true);
        console.log('Password submitted, processing...');
        passwordInput = input.value;
        await handleDecryptionOnLoad();
    } catch (error) {
        console.error('Password submission failed:', error);
        showStatus('验证密码时出错: ' + error.message, true);
        setLoading(false);
        passwordInput = null; // 重置密码以便重试
    }
}

// 添加密码处理相关的函数
async function deriveKeyFromPassword(password, salt) {
    console.log('Deriving key from password...');
    const encoder = new TextEncoder();
    const passwordBuffer = encoder.encode(password);
    
    const keyMaterial = await window.crypto.subtle.importKey(
        "raw",
        passwordBuffer,
        { name: "PBKDF2" },
        false,
        ["deriveBits", "deriveKey"]
    );
    
    return window.crypto.subtle.deriveKey(
        {
            name: "PBKDF2",
            salt: salt,
            iterations: 100000,
            hash: "SHA-256"
        },
        keyMaterial,
        { name: "AES-GCM", length: 256 },
        true,
        ["encrypt", "decrypt"]
    );
}

// --- Initial Setup ---
document.addEventListener('DOMContentLoaded', async () => {
    console.log("DOM fully loaded.");
    
    try {
        const configResponse = await fetch('/config');
        if (configResponse.ok) {
            const configData = await configResponse.json();
            if (configData.maxFileSizeMB) {
                maxFileSizeMB = parseInt(configData.maxFileSizeMB, 10);
                console.log('Max file size loaded from config:', maxFileSizeMB, 'MB');
            }
        }
    } catch (error) {
        console.error('Error fetching config:', error);
        showStatus('无法加载服务器配置: ' + error.message, true);
    }

    // 初始化密码保护功能
    initializePasswordProtection();

    // 处理解密逻辑
    if (window.location.search.includes('id=')) {
        handleDecryptionOnLoad();
    }
});

function initializePasswordProtection() {
    const enablePasswordCheckbox = document.getElementById('enablePassword');
    const passwordInputGroup = document.getElementById('passwordInputGroup');
    const accessPassword = document.getElementById('accessPassword');
    const passwordNote = document.getElementById('passwordNote');

    if (enablePasswordCheckbox && passwordInputGroup) {
        enablePasswordCheckbox.addEventListener('change', (e) => {
            passwordInputGroup.style.display = e.target.checked ? 'block' : 'none';
            if (!e.target.checked && accessPassword) {
                accessPassword.value = '';
                if (passwordNote) {
                    passwordNote.classList.add('hidden');
                }
            }
        });
    }

    // The onsubmit handler and handleFormSubmit function are removed.
    // Logic will be integrated into the addEventListener callback.
}

// handleFormSubmit function removed.

async function handleFileEncryption(masterKeyBase64, salt, encryptionKey, encryptedMasterKey) {
    const file = fileInput.files[0];
    if (!file) {
        throw new Error("请选择一个文件。");
    }

    const maxSizeBytes = maxFileSizeMB * 1024 * 1024;
    if (file.size > maxSizeBytes) {
        throw new Error(`文件大小超过限制 (${maxFileSizeMB} MB)`);
    }

    showStatus("正在读取并加密文件...");
    const dataBuffer = await file.arrayBuffer();

    // 记录原始文件大小用于验证
    const originalSize = dataBuffer.byteLength;

    // 加密文件内容
    showStatus("正在加密文件...");
    const { encryptedContent, iv } = await encryptData(dataBuffer, encryptionKey);

    // 验证加密后的大小是否合理（考虑AES-GCM的填充）
    const expectedOverhead = 16; // AES-GCM tag size
    if (Math.abs(encryptedContent.byteLength - (originalSize + expectedOverhead)) > 32) {
        throw new Error("加密后文件大小异常，请重试");
    }

    // 传递实际的加密后大小而不是原始大小
    await handleChunkUpload(file.name, encryptedContent.byteLength, encryptedContent, iv, salt, masterKeyBase64, encryptedMasterKey);
}

async function handleTextEncryption(masterKeyBase64, salt, encryptionKey, encryptedMasterKey) {
    const message = messageTextarea.value;
    if (!message.trim()) {
        throw new Error("请输入文本消息。");
    }

    showStatus("正在加密文本...");
    const dataBuffer = new TextEncoder().encode(message);
    
    try {
        const { encryptedContent, iv } = await encryptData(dataBuffer, encryptionKey);
        
        // 验证加密数据的完整性
        if (encryptedContent.byteLength < 16) { // 至少应该包含认证标签
            throw new Error("加密数据无效");
        }
        
        const encryptedBase64 = arrayBufferToBase64(encryptedContent);
        const ivBase64 = arrayBufferToBase64(iv);
        const saltBase64 = arrayBufferToBase64(salt);

        showStatus("正在将加密数据发送到服务器...");
        const payload = {
            encryptedData: encryptedBase64,
            iv: ivBase64,
            salt: saltBase64,
            passwordProtection: encryptedMasterKey
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