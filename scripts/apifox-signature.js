/**
 * 木雷消息服务 - Apifox 自动签名脚本
 * 
 * 使用说明:
 * 1. 在 Apifox 中创建公共脚本，将此代码粘贴进去
 * 2. 在环境变量中配置 APP_ID 和 APP_SECRET
 * 3. 在需要签名的接口「前置脚本」中调用此公共脚本
 * 
 * 签名算法:
 * - SignContent = Method + Path + SortedParams + Timestamp + Nonce
 * - Signature = Hex(HMAC-SHA256(SignContent, AppSecret))
 */

// 引入 crypto-js 库
const CryptoJS = require('crypto-js');

// ==================== 配置区域 ====================

// 从环境变量获取 APP_ID 和 APP_SECRET
const appId = pm.environment.get('APP_ID');
const appSecret = pm.environment.get('APP_SECRET');

// 检查必要的环境变量
if (!appId || !appSecret) {
    console.error('❌ 请在环境变量中配置 APP_ID 和 APP_SECRET');
    throw new Error('Missing APP_ID or APP_SECRET in environment variables');
}

// ==================== 签名生成 ====================

/**
 * 递归排序对象的所有 key（包括嵌套对象）
 * @param {any} obj - 要排序的对象
 * @returns {any} - 排序后的对象
 */
function sortObjectKeys(obj) {
    if (obj === null || typeof obj !== 'object') {
        return obj;
    }
    
    if (Array.isArray(obj)) {
        return obj.map(item => sortObjectKeys(item));
    }
    
    const sortedKeys = Object.keys(obj).sort();
    const result = {};
    for (const key of sortedKeys) {
        result[key] = sortObjectKeys(obj[key]);
    }
    return result;
}

/**
 * 获取排序后的请求参数字符串
 * @returns {string} - 排序后的 JSON 字符串，无参数时返回空字符串
 */
function getSortedParams() {
    const body = pm.request.body;
    
    if (!body || body.mode !== 'raw') {
        return '';
    }
    
    const rawBody = body.raw;
    if (!rawBody || rawBody.trim() === '') {
        return '';
    }
    
    try {
        const jsonData = JSON.parse(rawBody);
        const sortedData = sortObjectKeys(jsonData);
        return JSON.stringify(sortedData);
    } catch (e) {
        console.warn('⚠️ 请求体不是有效的 JSON 格式，跳过参数排序');
        return '';
    }
}

/**
 * 生成 UUID v4
 * @returns {string} - UUID 字符串
 */
function generateUUID() {
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
        const r = Math.random() * 16 | 0;
        const v = c === 'x' ? r : (r & 0x3 | 0x8);
        return v.toString(16);
    });
}

// 获取请求信息
const method = pm.request.method.toUpperCase();
const url = pm.request.url;
const path = url.getPath();

// 生成时间戳和随机字符串
const timestamp = Math.floor(Date.now() / 1000).toString();
const nonce = generateUUID();

// 获取排序后的参数
const sortedParams = getSortedParams();

// 构造签名内容: Method + Path + SortedParams + Timestamp + Nonce
const signContent = method + path + sortedParams + timestamp + nonce;

// 计算 HMAC-SHA256 签名
const signature = CryptoJS.HmacSHA256(signContent, appSecret).toString(CryptoJS.enc.Hex);

// ==================== 注入请求头 ====================

pm.request.headers.upsert({ key: 'X-App-Id', value: appId });
pm.request.headers.upsert({ key: 'X-Timestamp', value: timestamp });
pm.request.headers.upsert({ key: 'X-Nonce', value: nonce });
pm.request.headers.upsert({ key: 'X-Signature', value: signature });
pm.request.headers.upsert({ key: 'Content-Type', value: 'application/json' });

// ==================== 调试日志 ====================

console.log('🔐 签名信息:');
console.log('  Method:', method);
console.log('  Path:', path);
console.log('  Timestamp:', timestamp);
console.log('  Nonce:', nonce);
console.log('  SortedParams:', sortedParams || '(empty)');
console.log('  SignContent:', signContent);
console.log('  Signature:', signature);

