// 获取预先设置的 APPKEY
let key = pm.environment.get('APPKEY');

// 收集需要参与签名的参数
let param = {};

// 处理 Query 参数
pm.request.url.query.each(item => {
  if (!item.disabled && item.value !== '') {
    param[item.key] = item.value;
  }
});

// 处理 Body 参数
if (pm.request.body) {
  let bodyData;
  switch (pm.request.body.mode) {
    case 'formdata':
      bodyData = pm.request.body.formdata;
      break;
    case 'urlencoded':
      bodyData = pm.request.body.urlencoded;
      break;
    case 'raw': {
      const contentType = pm.request.headers.get('content-type');
      if (contentType && contentType.toLowerCase().includes('application/json')) {
        try {
          const jsonData = JSON.parse(pm.request.body.raw);
          for (let key in jsonData) {
            if (jsonData[key] !== '') {
              param[key] = jsonData[key];
            }
          }
        } catch (e) {
          console.log('请求 body 不是 JSON 格式');
        }
      }
      break;
    }
    default:
      break;
  }
  if (bodyData) {
    bodyData.each(item => {
      if (!item.disabled && item.value !== '') {
        param[item.key] = item.value;
      }
    });
  }
}

// 排序参数
const sortedKeys = Object.keys(param).filter(key => key !== 'sign').sort();
const paramString = sortedKeys.map(key => `${key}=${encodeURIComponent(param[key])}`).join('&');
const stringSignTemp = `${paramString}&key=${key}`;

// 生成签名
const sign = CryptoJS.MD5(stringSignTemp).toString().toUpperCase();

// 注入签名到请求中
pm.request.url.query.upsert({ key: 'sign', value: sign });