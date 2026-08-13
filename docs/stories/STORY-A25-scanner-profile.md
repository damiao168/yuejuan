# STORY-A25：Scanner Integration 与设备 Profile

状态：Implemented（当前工作树）

## 范围

- 桌面端维护设备指纹、DPI、单双面、色彩、纸张、旋转、压缩和模板 Profile。
- 开始采集前进行设备/Profile/网络/磁盘预检，并提供样张质量提示。

## 接线

- `scanner.rs` 与 `scannerProfile.ts` 隔离 WIA 设备发现与 Profile 事实；React 不绑定厂商 SDK。
- 扫描页面要求先选择通过预检的 Profile，再将 PDF、JPEG、PNG 或 TIFF 文件进入耐久 Spool。

## 验证

- Rust 预检和 Profile 测试、Desktop 类型/构建测试已执行。

## 外部边界

- 当前设备发现不承诺所有 WIA/TWAIN 机型的直接采集；实际驱动、进纸、双面和样张质量必须逐设备验证。
