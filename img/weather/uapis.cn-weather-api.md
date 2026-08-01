# uapis.cn 天气 API 文档

## 概述

uapis.cn 提供完全免费的天气查询 API 接口，**无需注册、无需 API 密钥**即可调用。支持国内 3000+ 城市，涵盖省、市、区县级别。

---

## 基本信息

| 项目 | 说明 |
|:---|:---|
| **API 地址** | `https://uapis.cn/api/v1/misc/weather` |
| **请求方式** | GET |
| **响应格式** | JSON |
| **认证方式** | 无需认证 |
| **使用限制** | 免费使用 |

---

## 请求参数

| 参数名 | 类型 | 必填 | 说明 |
|:---|:---|:---|:---|
| `city` | string | 是 | 城市名称，支持中文（如 "北京"、"上海"、"福田区"） |

> **注意**：不传 `city` 参数时，API 会自动根据请求来源 IP 进行定位。

---

## 调用示例

### cURL

```bash
# 查询北京天气
curl 'https://uapis.cn/api/v1/misc/weather?city=北京'

# 查询上海天气
curl 'https://uapis.cn/api/v1/misc/weather?city=上海'

# 查询区县级别（如深圳福田区）
curl 'https://uapis.cn/api/v1/misc/weather?city=福田区'
```

### JavaScript (Fetch)

```javascript
// 查询天气
async function getWeather(city) {
  const response = await fetch(`https://uapis.cn/api/v1/misc/weather?city=${encodeURIComponent(city)}`);
  const data = await response.json();
  return data;
}

// 使用示例
getWeather('北京').then(data => {
  console.log(`当前温度: ${data.temperature}°C`);
  console.log(`天气状况: ${data.weather}`);
});
```

### Python

```python
import requests

def get_weather(city):
    url = 'https://uapis.cn/api/v1/misc/weather'
    params = {'city': city}
    response = requests.get(url, params=params)
    return response.json()

# 使用示例
data = get_weather('上海')
print(f"城市: {data['city']}")
print(f"温度: {data['temperature']}°C")
print(f"天气: {data['weather']}")
```

---

## 响应格式

```json
{
  "province": "陕西省",
  "city": "渭南市",
  "adcode": "610500",
  "weather": "晴",
  "weather_icon": "100",
  "temperature": 9.9,
  "wind_direction": "微风",
  "wind_power": "",
  "humidity": 83,
  "report_time": "2026-03-10 23:27:02"
}
```

---

## 字段说明

| 字段名 | 类型 | 说明 |
|:---|:---|:---|
| `province` | string | 省份/直辖市名称 |
| `city` | string | 城市名称 |
| `adcode` | string | 行政区划代码（6位数字） |
| `weather` | string | 天气状况（如：晴、多云、小雨等） |
| `weather_icon` | string | 天气图标代码，用于匹配对应的天气图标 |
| `temperature` | number | 实时温度（单位：°C） |
| `wind_direction` | string | 风向（如：北风、东风、微风） |
| `wind_power` | string | 风力等级（部分城市可能为空） |
| `humidity` | number | 相对湿度（单位：百分比 %） |
| `report_time` | string | 数据报告时间（格式：YYYY-MM-DD HH:mm:ss） |

---

## 天气图标代码对照表

| weather_icon | 天气状况 | 推荐图标 |
|:---|:---|:---|
| `100` | 晴 | `weather-sunny.svg` |
| `101` | 多云 | `weather-cloudy.svg` |
| `102` | 阴 | `weather-cloudy.svg` |
| `103` | 雷阵雨 | `weather-lightning-rainy.svg` |
| `104` | 雷阵雨冰雹 | `weather-lightning.svg` |
| `200` | 阵风 | `weather-windy.svg` |
| `301` | 雨 | `weather-rainy.svg` |
| `302` | 冻雨 | `weather-pouring.svg` |
| `303` | 雷阵雨 | `weather-lightning-rainy.svg` |
| `304` | 雷阵雨冰雹 | `weather-lightning.svg` |
| `307` | 中雨 | `weather-rainy.svg` |
| `308` | 大雨 | `weather-pouring.svg` |
| `309` | 毛毛雨 | `weather-partly-rainy.svg` |
| `310` | 暴雨 | `weather-pouring.svg` |
| `311` | 大暴雨 | `weather-pouring.svg` |
| `312` | 特大暴雨 | `weather-pouring.svg` |
| `401` | 雪 | `weather-snowy.svg` |
| `402` | 小雪 | `weather-snowy.svg` |
| `403` | 中雪 | `weather-snowy.svg` |
| `404` | 大雪 | `weather-snowy-heavy.svg` |
| `405` | 暴雪 | `weather-snowy-heavy.svg` |
| `406` | 雨夹雪 | `weather-snowy-rainy.svg` |
| `407` | 阵雪 | `weather-snowy.svg` |
| `408` | 夜间阵雪 | `weather-night.svg` |
| `409` | 冻雨 | `weather-snowy-rainy.svg` |
| `410` | 雾 | `weather-fog.svg` |
| `456` | 夜间有雨 | `weather-night-partly-cloudy.svg` |
| `457` | 夜间多云 | `weather-night-partly-cloudy.svg` |
| `499` | 未知 | `weather-cloudy.svg` |
| `500` | 雾 | `weather-fog.svg` |
| `502` | 扬沙 | `weather-dust.svg` |
| `503` | 浮尘 | `weather-dust.svg` |
| `507` | 沙尘暴 | `weather-tornado.svg` |
| `999` | 未知 | `weather-cloudy.svg` |

> **提示**：本目录下的 `img/` 文件夹中包含所有天气图标 SVG 文件，可根据 `weather_icon` 字段值选择对应图标。

---

## 完整使用示例

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <title>天气查询示例</title>
</head>
<body>
  <div id="weather">
    <h2 id="city">加载中...</h2>
    <p id="temp"></p>
    <p id="desc"></p>
    <img id="icon" alt="天气图标">
  </div>

  <script>
    async function loadWeather() {
      const city = '北京';
      const res = await fetch(`https://uapis.cn/api/v1/misc/weather?city=${city}`);
      const data = await res.json();

      document.getElementById('city').textContent = data.city;
      document.getElementById('temp').textContent = `${data.temperature}°C`;
      document.getElementById('desc').textContent = `${data.weather} | ${data.wind_direction} | 湿度 ${data.humidity}%`;
      document.getElementById('icon').src = `./img/weather/weather-sunny.svg`; // 根据 weather_icon 动态选择
    }

    loadWeather();
  </script>
</body>
</html>
```

---

## 错误处理

API 可能返回错误信息，常见错误码：

| 错误信息 | 说明 |
|:---|:---|
| `{"error": "City not found"}` | 城市不存在或名称错误 |
| `{"error": "Service unavailable"}` | 服务暂时不可用 |

建议在调用时添加错误处理：

```javascript
async function getWeather(city) {
  try {
    const response = await fetch(`https://uapis.cn/api/v1/misc/weather?city=${encodeURIComponent(city)}`);
    if (!response.ok) {
      throw new Error(`HTTP error: ${response.status}`);
    }
    const data = await response.json();
    if (data.error) {
      throw new Error(data.error);
    }
    return data;
  } catch (error) {
    console.error('获取天气失败:', error.message);
  }
}
```

---

## 相关资源

- **官方网站**: [https://uapis.cn](https://uapis.cn)
- **API 文档**: [https://uapis.cn/docs](https://uapis.cn/docs)
- **天气图标库**: 本目录下的 `about-weather-ico.md`
