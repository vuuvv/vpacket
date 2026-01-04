# 协议配置文档说明

本文档详细说明了 `protocols.yaml` 协议配置文件中各个字段的含义和使用方法。

## 文件概述

`protocols.yaml` 是一个协议定义配置文件，用于定义与设备通信的协议格式。该文件包含两种主要协议：
1. **CustomBoardBinary** - 二进制协议，用于与自定义开发板通信
2. **DTUHeartbeat** - 文本协议，用于DTU设备心跳检测

---

## 1. 数据结构库 (Data Structures Library)

数据结构库定义了可复用的数据结构，用于在多个协议中共享。

### 1.1 ResponseCode 结构
用于表示标准响应码的结构：
```yaml
ResponseCode:
  fields:
    - name: "data.code"      # 响应码，0表示成功
    - name: "data.success"   # 成功标志，通过计算得出 (fields.data.code==0)
```

### 1.2 EmptyResponse 结构
用于表示空响应，包含额外的空标志：
```yaml
EmptyResponse:
  fields:
    - name: "data.code"      # 响应码
    - name: "data.success"   # 成功标志
    - name: "data.empty"     # 空标志，始终为true
```

### 1.3 Unknown_Payload 结构
用于处理无法识别的指令的默认结构：
```yaml
Unknown_Payload:
  fields:
    - name: "raw_data_payload"  # 原始数据负载，以十六进制格式存储
```

---

## 2. CustomBoardBinary 协议详解

这是与自定义开发板通信的二进制协议。

### 2.1 协议帧格式 (Framing Rule)

| 字段 | 位置 | 长度 | 说明 |
|------|------|------|------|
| Header Marker | 0-1 | 2字节 | 帧头标记，固定值 '7273' |
| Direction | 2 | 1字节 | 方向标志，默认 'AA' |
| SN | 3-14 | 12字节 | 序列号 |
| Product Code | 15-16 | 2字节 | 产品代码，默认 '0000' |
| Version | 17-18 | 2字节 | 版本号，默认 '0000' |
| Command | 19 | 1字节 | 命令码 |
| Data Length | 20-21 | 2字节 | 数据长度 |
| Data | 22-(N-3) | 变长 | 数据负载 |
| CRC16 | (N-2)-(N-1) | 2字节 | CRC16校验码 |

**帧格式配置参数：**
- `header_marker`: "7273" - 帧头标识符
- `min_header_size`: 24 - 最小帧头大小
- `length_offset`: 20 - 长度字段在帧中的偏移量
- `length_size`: 2 - 长度字段的大小（字节）
- `length_adjustment`: 24 - 长度调整值

### 2.2 通用字段定义

| 字段名 | 类型 | 大小 | 说明 |
|--------|------|------|------|
| subDevice | calc | - | 标记为子设备（始终为true）|
| magic | string | 2字节 | 魔数，检查是否为'7273' |
| direction | string | 1字节 | 方向，默认'AA' |
| sn | string | 12字节 | 序列号 |
| productCode | string | 2字节 | 产品代码，默认'0000' |
| versionRaw | string | 2字节 | 原始版本号，默认'0000' |
| command | string | 1字节 | 命令码 |
| dataLen | int | 2字节 | 数据长度（解码时）/ 计算得出（编码时）|
| dataCrc | crc16 | 2字节 | CRC16校验码 |

### 2.3 命令码详解

#### 命令 0x01: 读取IO状态
- **方向**: 服务器→设备
- **功能**: 读取设备的IO端口状态
- **解码响应**:
  - `data.response.code`: 响应码
  - `data.error`: 错误信息（失败时）
  - `data.ioState`: IO状态数组，每个元素包含port(端口)和value(值)

#### 命令 0x02: 输出IO
- **方向**: 服务器→设备
- **功能**: 控制设备的IO输出
- **编码字段**:
  - `data.gpio`: GPIO端口号
  - `data.value`: 输出值
- **解码响应**:
  - `data.response.code`: 响应码
  - `data.error`: 错误信息（失败时）

#### 命令 0x04: 设置时间戳
- **方向**: 服务器→设备
- **功能**: 设备时间同步
- **编码字段**:
  - `data.timestamp`: 时间戳（Unix时间戳）
- **解码响应**:
  - `data.response.code`: 响应码
  - `data.error`: 错误信息（失败时）

#### 命令 0x05: 播放语音
- **方向**: 服务器→设备
- **功能**: 控制设备播放指定语音
- **编码字段**:
  - `data.index`: 语音索引号
- **解码响应**:
  - `data.response.code`: 响应码
  - `data.error`: 错误信息（失败时）

#### 命令 0x0D: 设置LED显示方向
- **方向**: 服务器→设备
- **功能**: 设置LED显示屏的方向
- **编码字段**:
  - `data.direction`: 显示方向
- **解码响应**:
  - `data.response.code`: 响应码
  - `data.error`: 错误信息（失败时）

#### 命令 0x0E: 显示文字
- **方向**: 服务器→设备
- **功能**: 在LED显示屏上显示文字
- **编码字段**:
  - `data.duration`: 显示持续时间
  - `data.line1Length`: 第1行文字长度
  - `data.line1Speed`: 第1行滚动速度
  - `data.line2Length`: 第2行文字长度
  - `data.line2Speed`: 第2行滚动速度
  - `data.line3Length`: 第3行文字长度
  - `data.line3Speed`: 第3行滚动速度
  - `data.line4Length`: 第4行文字长度
  - `data.line4Speed`: 第4行滚动速度
  - `data.content`: 文字内容（十六进制编码）
- **解码响应**:
  - `data.response.code`: 响应码
  - `data.error`: 错误信息（失败时）

#### 命令 0xC1: 设备上报ID
- **方向**: 设备→服务器
- **功能**: 设备主动上报设备ID
- **解码字段**:
  - `data.code`: 设备ID（长度由dataLen决定）

#### 命令 0xC2: 设备上报IO状态
- **方向**: 设备→服务器
- **功能**: IO状态变化时设备主动上报
- **解码字段**:
  - `data.ioState`: IO状态数组，每个元素包含port(端口)和value(值)
- **编码字段**:
  - `data.code`: 确认码

#### 命令 0xC3: 刷卡信息上报
- **方向**: 设备→服务器
- **功能**: 刷卡事件上报
- **解码字段**:
  - `data.channel`: 刷卡通道
  - `data.idLen`: ID长度
  - `data.cardNo`: 卡号
  - `data.userRole`: 用户角色
  - `data.room`: 房间号
  - `data.startTime`: 开始时间
  - `data.endTime`: 结束时间
- **编码字段**:
  - `data.code`: 确认码

#### 命令 0xFE: 读取设备ID
- **方向**: 服务器→设备
- **功能**: 查询设备ID
- **解码响应**:
  - `data.success`: 成功标志

---

## 3. DTUHeartbeat 协议详解

这是DTU设备的文本格式心跳协议。

### 3.1 协议格式

DTU心跳包格式：`[SN:序列号]`

**帧格式配置参数：**
- `start_delimiter`: "s'['" - 开始分隔符 '['
- `end_delimiter`: "s']'" - 结束分隔符 ']'
- `max_len`: 128 - 最大长度

### 3.2 字段定义

| 字段名 | 类型 | 计算公式 | 说明 |
|--------|------|----------|------|
| sn | string | 实际序列号 | 从协议中提取的序列号 |
| command | calc | 'heartbeat' | 固定为'heartbeat' |
| deviceType | calc | 'dtu' | 固定为'dtu' |

---

## 4. 字段类型说明

### 4.1 基本类型
- `uint`: 无符号整数
- `string`: 字符串
- `hex`: 十六进制数据
- `calc`: 计算字段，通过公式得出值

### 4.2 复合类型
- `array`: 数组类型，包含多个相同结构的元素
- `struct`: 结构类型，引用预定义的数据结构

### 4.3 特殊属性
- `size`: 字段大小（字节）
- `size_expr`: 大小表达式，用于动态计算
- `default`: 默认值
- `crc`: CRC校验类型
- `crc_start`: CRC校验开始位置
- `crc_end`: CRC校验结束位置
- `flow`: 数据流方向（encode/decode）
- `round`: 轮次，用于多次处理场景

---

## 5. 使用示例

### 5.1 发送读取IO状态命令
```
Frame: 7273 AA 123456789012 0000 0000 01 0000 [CRC16] [空数据]
```

### 5.2 设备上报IO状态
```
Frame: 7273 AA 123456789012 0000 0000 C2 0004 [端口1:值1][端口2:值2] [CRC16]
```

### 5.3 DTU心跳包
```
Frame: [1234567890]
```

---

## 6. 注意事项

1. **字节序**: 所有多字节数值采用大端序（Big Endian）
2. **CRC校验**: 使用CRC16-Modbus算法
3. **字符串编码**: 文本字段使用ASCII编码
4. **错误处理**: 所有命令都包含错误信息字段，用于故障诊断
5. **兼容性**: Unknown_Payload结构用于处理未知命令，确保协议的向后兼容性

---

*本文档最后更新：根据 protocols.yaml 协议配置生成*