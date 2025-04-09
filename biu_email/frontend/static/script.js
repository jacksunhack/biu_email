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

        if (isFileMode) {
            // --- File Mode: Encrypt then Chunk Upload ---
            const file = fileInput.files[0];
            if (!file) {
                throw new Error("请选择一个文件。");
            }
            const maxSizeBytes = maxFileSizeMB * 1024 * 1024;
            if (file.size > maxSizeBytes) {
                throw new Error('文件大小超过限制 (' + maxFileSizeMB + ' MB)。');
            }

            showStatus("正在读取文件...");
            const dataBuffer = await file.arrayBuffer();

            showStatus("正在加密文件 (这可能需要一些时间)...");
            // Encrypt the entire file content first
            const { encryptedContent, iv } = await encryptData(dataBuffer, encryptionKey);

            // Now handle the chunk upload of the encrypted content
            await handleChunkUpload(file.name, file.size, encryptedContent, iv, salt, masterKeyBase64);

        } else {
            // --- Text Mode: Use original /api/store ---
            const message = messageTextarea.value;
            if (!message.trim()) {
                throw new Error("请输入文本消息。");
            }
            const dataBuffer = new TextEncoder().encode(message);
            showStatus("正在加密文本...");

            const { encryptedContent, iv } = await encryptData(dataBuffer, encryptionKey);

            const encryptedBase64 = arrayBufferToBase64(encryptedContent);
            const ivBase64 = arrayBufferToBase64(iv);
            const saltBase64 = arrayBufferToBase64(salt);

            showStatus("正在将加密数据发送到服务器...");

            const payload = {
                encryptedData: encryptedBase64, // Send full encrypted data for text
                iv: ivBase64,
                salt: saltBase64,
            };

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
        }

    } catch (error) {
        console.error("处理过程中出错:", error);
        showStatus('错误: ' + error.message, true);
    } finally {
        // setLoading(false); // handleChunkUpload will manage loading state for file uploads
        if (!isFileMode) { // Only reset loading if it was text mode
            setLoading(false);
        }
    }
});

// --- Chunk Upload Functions ---
async function handleChunkUpload(originalFilename, originalFilesize, encryptedFileBuffer, iv, salt, masterKeyBase64) {
    showStatus("正在初始化分片上传...");

    // 1. Initialize Upload
    let uploadId;
    try {
        const initResponse = await fetch('/api/upload/init', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ fileName: originalFilename, fileSize: originalFilesize }) // Send original size
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
    } catch (error) {
        showStatus('错误: ' + error.message, true);
        setLoading(false);
        return; // Stop upload process
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
    await finalizeUpload(uploadId, iv, salt, originalFilename, masterKeyBase64);
}

async function finalizeUpload(uploadId, iv, salt, originalFilename, masterKeyBase64) {
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
                    originalFilename: originalFilename
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
    console.log('handleDecryptionOnLoad triggered.'); // <-- 添加日志 1
    const urlParams = new URLSearchParams(window.location.search);
    const dataId = urlParams.get('id');
    const masterKeyBase64 = window.location.hash.substring(1); // Get key from URL fragment

    if (dataId && masterKeyBase64) {
        console.log('Found dataId:', dataId, 'and masterKeyBase64:', masterKeyBase64 ? 'present' : 'missing'); // <-- 添加日志 2
        hideMessages();
        contentAreaDiv.classList.remove('hidden');
        decryptedContentDiv.innerHTML = '正在获取元数据...';
        setLoading(true); // Use loader visually

        let metadata;
        let iv;
        let salt;
        let originalFilename;

         try {
            // 1. Fetch data from /api/data/:id - This endpoint now returns different structures based on content type
            decryptedContentDiv.innerHTML = '正在获取数据...';
            const response = await fetch('/api/data/' + dataId);
            if (!response.ok) {
                if (response.status === 404) {
                    throw new Error("数据未找到或已被销毁。");
                }
                const errorData = await response.json().catch(() => ({ error: '无法解析服务器响应' }));
                throw new Error('获取数据时服务器错误 (' + response.status + '): ' + (errorData.error || response.statusText));
            console.log('Received data from /api/data:', JSON.stringify(responseData)); // <-- Log received data
            }
            const responseData = await response.json();

            // 2. Check response format to determine if it's text or file metadata
            // Check for lowercase keys based on Go struct 'json' tags for StoredData
            if (responseData.encryptedData && responseData.iv && responseData.salt) { // Check lowercase keys
                // --- Handle Text Message Decryption ---
                decryptedContentDiv.innerHTML = '正在解密文本消息...';
                iv = base64ToArrayBuffer(responseData.iv); // Use lowercase
                salt = base64ToArrayBuffer(responseData.salt); // Use lowercase
                const encryptedData = base64ToArrayBuffer(responseData.encryptedData); // Use lowercase

                // 3. Decrypt Text
                const encryptionKey = await deriveEncryptionKey(masterKeyBase64, salt);
                const decryptedBuffer = await decryptData(encryptedData, iv, encryptionKey);

                // 4. Display Text
                const decryptedText = new TextDecoder().decode(decryptedBuffer);
                decryptedContentDiv.textContent = decryptedText;
                decryptedContentDiv.innerHTML += "<br><br><small>消息将在销毁后从服务器删除。</small>";

                // 5. Burn Text Data
                try {
                    await fetch('/api/burn/' + dataId, { method: 'POST' });
                    console.log("Burn request sent for text data ID:", dataId);
                } catch (burnError) {
                    console.warn("发送销毁文本请求失败:", burnError);
                    decryptedContentDiv.innerHTML += "<br><strong style='color:orange;'>警告：无法自动销毁服务器上的文本数据。</strong>";
                }

            } else if (responseData.iv && responseData.salt) { // Check lowercase keys for metadata too
                // --- Handle File Download and Decryption ---
                // Response might contain metadata with lowercase keys if StoredMetadata isn't used or marshaled differently
                decryptedContentDiv.innerHTML = '获取到文件元数据，正在下载加密文件...';
                iv = base64ToArrayBuffer(responseData.iv); // Use lowercase
                salt = base64ToArrayBuffer(responseData.salt); // Use lowercase
                originalFilename = responseData.originalFilename; // Use lowercase

                // 3. Fetch Encrypted File Content
                const downloadResponse = await fetch('/api/download/' + dataId); // Assume /api/download handles file retrieval
                if (!downloadResponse.ok) {
                    if (downloadResponse.status === 404) {
                        throw new Error("加密文件内容未找到或已被销毁。");
                    }
                    throw new Error('下载加密文件时服务器错误 (' + downloadResponse.status + '): ' + downloadResponse.statusText);
                }

                decryptedContentDiv.innerHTML = '文件下载完成，正在解密...';
                const encryptedFileBuffer = await downloadResponse.arrayBuffer();

                // 4. Decrypt File
                const encryptionKey = await deriveEncryptionKey(masterKeyBase64, salt);
                const decryptedBuffer = await decryptData(encryptedFileBuffer, iv, encryptionKey);

                // 5. Trigger File Download
                const filenameToUse = originalFilename || ('decrypted_file_' + dataId + '.bin'); // Fallback filename
                decryptedContentDiv.innerHTML = '文件已解密: <strong>' + filenameToUse + '</strong><br>准备下载...';
                const blob = new Blob([decryptedBuffer]);
                const url = URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = filenameToUse;
                console.log('Triggering download for:', filenameToUse);
                document.body.appendChild(a);
                a.click();
                document.body.removeChild(a);
                URL.revokeObjectURL(url);
                decryptedContentDiv.innerHTML += "<br>下载已开始。文件将在下载后从服务器销毁。";

                // 6. Burn File Data
                try {
                    // Assuming the same burn endpoint works for data stored via file flow
                    await fetch('/api/burn/' + dataId, { method: 'POST' });
                    console.log("Burn request sent for file data ID:", dataId);
                } catch (burnError) {
                    console.warn("发送销毁文件请求失败:", burnError);
                    decryptedContentDiv.innerHTML += "<br><strong style='color:orange;'>警告：无法自动销毁服务器上的文件数据。</strong>";
                }

            } else {
                // Invalid response format from /api/data/:id
                throw new Error("从服务器接收到的数据格式无效或不完整。");
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