# Fields 字段规范详细说明

本文档详细说明了 JSON 数据输入中`fields`部分的字段规范和具体要求。

## 1. Fields 字段结构概述

`fields`对象包含了协议编码/解码所需的所有字段数据，其结构根据命令类型的不同而有所变化。

### 1.1 通用字段

所有命令都必须包含的字段：

```json
{
  "fields": {
    "command": "命令码（十六进制字符串）"
  }
}
```

## 2. 各命令 Fields 字段详解

### 2.1 命令 0x01: 读取 IO 状态

**Fields 结构**：

```json
{
  "sn": "xxxx",
  "command": "01"
}
```

**说明**：此命令无需额外字段，仅需命令码即可。

### 2.2 命令 0x02: 输出 IO

**Fields 结构**：

```json
{
  "sn": "xxxx",
  "command": "02",
  "data": {
    "gpio": "GPIO端口号（uint，范围0-255）",
    "value": "输出值（uint，0或1）"
  }
}
```

**字段说明**：

- `data.gpio`: GPIO 端口号，必须为 0-255 的整数
- `data.value`: 输出值，只能为 0（低电平）或 1（高电平）

**示例**：

```json
{
  "sn": "xxxx",
  "command": "02",
  "data": {
    "gpio": 5,
    "value": 1
  }
}
```

### 2.3 命令 0x04: 设置时间戳

**Fields 结构**：

```json
{
  "sn": "xxxx",
  "command": "04",
  "data": {
    "timestamp": "Unix时间戳（uint，秒）"
  }
}
```

**字段说明**：

- `data.timestamp`: Unix 时间戳，32 位无符号整数，范围 0-4294967295

**示例**：

```json
{
  "sn": "xxxx",
  "command": "04",
  "data": {
    "timestamp": 1634567890
  }
}
```

### 2.4 命令 0x05: 播放语音

**Fields 结构**：

```json
{
  "sn": "xxxx",
  "command": "05",
  "data": {
    "index": "语音索引号（uint，范围0-255）"
  }
}
```

**字段说明**：

- `data.index`: 语音文件的索引号，设备预存的语音文件编号

**示例**：

```json
{
  "sn": "xxxx",
  "command": "05",
  "data": {
    "index": 3
  }
}
```

### 2.5 命令 0x0D: 设置 LED 显示方向

**Fields 结构**：

```json
{
  "sn": "xxxx",

  "command": "0D",
  "data": {
    "direction": "显示方向（uint）"
  }
}
```

**字段说明**：

- `data.direction`: LED 显示方向，通常 0=正常，1=翻转

**示例**：

```json
{
  "sn": "xxxx",
  "command": "0D",
  "data": {
    "direction": 0
  }
}
```

### 2.6 命令 0x0E: 显示文字

**Fields 结构**：

```json
{
  "sn": "xxxx",
  "command": "0E",
  "data": {
    "duration": "显示持续时间（uint，秒）",
    "line1Length": "第1行文字长度（uint）",
    "line1Speed": "第1行滚动速度（uint）",
    "line2Length": "第2行文字长度（uint）",
    "line2Speed": "第2行滚动速度（uint）",
    "line3Length": "第3行文字长度（uint）",
    "line3Speed": "第3行滚动速度（uint）",
    "line4Length": "第4行文字长度（uint）",
    "line4Speed": "第4行滚动速度（uint）",
    "content": "文字内容（hex字符串）"
  }
}
```

**字段说明**：

- `data.duration`: 显示持续时间，0=持续显示，>0=显示指定秒数
- `data.line1Length`: 第 1 行文字的字节数
- `data.line1Speed`: 第 1 行滚动速度，值越大滚动越快
- `data.line2Length`: 第 2 行文字的字节数（0 表示不显示）
- `data.line2Speed`: 第 2 行滚动速度
- `data.line3Length`: 第 3 行文字的字节数（0 表示不显示）
- `data.line3Speed`: 第 3 行滚动速度
- `data.line4Length`: 第 4 行文字的字节数（0 表示不显示）
- `data.line4Speed`: 第 4 行滚动速度
- `data.content`: 文字内容的十六进制编码，每行内容按顺序连接

**示例**：

```json
{
  "sn": "xxxx",
  "command": "0E",
  "data": {
    "duration": 10,
    "line1Length": 12,
    "line1Speed": 50,
    "line2Length": 6,
    "line2Speed": 75,
    "line3Length": 0,
    "line3Speed": 0,
    "line4Length": 0,
    "line4Speed": 0,
    "content": "E59CA8E5B88BE5B095E5A4A9E5A4A9"
  }
}
```

### 2.7 命令 0xC1: 设备上报 ID（解码响应）

**Fields 结构**：

```json
{
  "sn": "xxxx",
  "command": "C1",
  "data": {
    "code": "设备ID（uint）"
  }
}
```

**说明**：此为设备响应的解码格式。

### 2.8 命令 0xC2: 设备上报 IO 状态

**Fields 结构**：

```json
{
  "sn": "xxxx",
  "command": "C2",
  "data": {
    "code": "确认码（uint）",
    "ioState": [
      {
        "port": "端口号（uint）",
        "value": "端口值（uint）"
      }
    ]
  }
}
```

**字段说明**：

- `data.ioState`: IO 状态数组，每个元素包含 port 和 value
- `data.code`: 服务器确认码

**示例**：

```json
{
  "sn": "xxxx",
  "command": "C2",
  "data": {
    "code": 0,
    "ioState": [
      { "port": 1, "value": 1 },
      { "port": 2, "value": 0 },
      { "port": 3, "value": 1 }
    ]
  }
}
```

### 2.9 命令 0xC3: 刷卡信息上报

**Fields 结构**：

```json
{
  "sn": "xxxx",
  "command": "C3",
  "data": {
    "channel": "刷卡通道（uint）",
    "idLen": "ID长度（uint）",
    "cardNo": "卡号（string）",
    "userRole": "用户角色（hex）",
    "room": "房间号（hex）",
    "startTime": "开始时间（hex）",
    "endTime": "结束时间（hex）",
    "code": "确认码（uint）"
  }
}
```

**字段说明**：

- `data.channel`: 刷卡通道编号
- `data.idLen`: 卡号长度
- `data.cardNo`: 卡号字符串
- `data.userRole`: 用户角色，十六进制格式
- `data.room`: 房间号，十六进制格式
- `data.startTime`: 有效期开始时间，十六进制格式
- `data.endTime`: 有效期结束时间，十六进制格式
- `data.code`: 确认码

**示例**：

```json
{
  "sn": "xxxx",
  "command": "C3",
  "data": {
    "channel": 1,
    "idLen": 8,
    "cardNo": "12345678",
    "userRole": "01",
    "room": "010203",
    "startTime": "20220101",
    "endTime": "20231231",
    "code": 0
  }
}
```

### 2.10 命令 0xFE: 读取设备 ID

**Fields 结构**：

```json
{
  "sn": "xxxx",
  "command": "FE",
  "data": {
    "success": "成功标志（uint，0或1）"
  }
}
```

**说明**：此为设备响应的解码格式。

## 3. 字段验证规则

### 3.1 数据类型验证

- **uint 字段**: 必须为非负整数
- **string 字段**: 必须为字符串类型
- **hex 字段**: 必须为大写十六进制字符串，不带 0x 前缀

### 3.2 范围验证

- GPIO 端口号: 0, 1, 161, 162
- 输出值: 0,1,2
- 时间戳: 0-4294967295 (32 位无符号整数)
- 语音索引: 0-255

---

_本文档最后更新：Fields 字段规范详细说明_
