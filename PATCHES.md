# 私有补丁与维护契约

来源：gen2brain/gav1d v0.2.7，commit `0d06c9dc16a45726fec53e2f5f63cf1ce61d6da9`。
原始 COPYING、PATENTS 原样保留；仓库历史可追溯该提交。
用户于 2026-11-14 在 V05D / APP-2656 确认私有最小补丁镜像。

补丁范围：

1. module/import 机械迁至 github.com/appforges/gav1d。
2. 从 toImage 抽取共享 pictureToImage；公开 PictureToNRGBA 只接受真实、仍存活的 8-bit I420/I444、CICP 1/13/6 Picture。参数只读借用，返回不别名输入的 opaque NRGBA；不解码、不应用容器变换或 ICC、不 Release/Flush。v0.2.7-boundlet.1 保持原RGB算法；v0.2.7-boundlet.2 的 V05T 修正仅该显式入口的 R/B 乘加舍入，普通 Decode 的原默认算法和 SIMD 不变。
3. Decoder.DisableScratchPooling 显式 true 时 frameContext 不取得 tc/fb pool 引用，不 Get/Put 缓存；false 保持原行为。picture pool 不在本选项范围，仍须调用者按生命周期排空、释放并丢弃 decoder。
4. 测试与本地双 Go 脚本；GitHub Actions 暂停，保留原工作流。

### V05T 舍入差异复核（2026-11-16）

新增独立几何参考首次31/32精确通过，255×255/limited/I420仅(49,236)红通道49而参考50。原AVIF解出的全部97793字节Y/U/V与独立libaom相同，libavif分别使用libaom/dav1d的RGB参考相同；不是码流解码或缩放差异。固定参考C实现在R/B乘加处可融合，而原Go/SIMD显式分两步，恰在49.5阈值两侧。原图和金标准不变，最小见证与工具版本/二进制SHA见testdata/boundlet/v05t-provenance.json。

本补丁仅PictureToNRGBA显式入口选择纯Go行函数：R/B用float64 math.FMA再显式转float32，chroma bilinear、系数、range与最终8位取整保持；不使用隐式编译器融合。普通Decode继续原函数/SIMD/舍入，原5个上游颜色失败不能冒修复。新增真实见证测试同时检查修正入口全像素匹配与原入口单像素仍49；Boundlet调用方须按新固定版本/新Profile记录，不能将旧v1证据改写为新算法已验。只据已验证样本声称保真，不宣称所有C编译器/架构的RGB字节相同。没有提高格式/资源上限或替换AV1解码器。

独立 module 可按固定版本获取，不使用宿主本地 replace/go.work。每次升级记录上游 SHA、补丁差异和模块摘要，复验两 Go、noasm、精确 RGB、所有权、取消与资源证据；发现新差异时另行评审。上游提供等价能力后可以评审迁回，不自动切换。

这不是完整 AVIF 安全准入：无硬 RSS/CPU 或立即内存归还保证，不提供 context 强制中断，不提高 Boundlet 默认额度或启用生产 AVIF。独立验收位于 Boundlet 的 tools/avif-admission。

本地脚本默认执行离线测试，明确不启用本机原生 avifdec；可显式设置 AVIFDEC_BIN 单独对照。当前原生 libavif 1.4.2/libaom 3.14.1/no-libyuv 对未修改上游和本镜像的 TestColor 均有相同五个失败：clap-10、thumb、yuv420-10、yuv422-12、yuv444-10。失败记录保留，不修改测试或算法掩盖；不将离线通过称为这些原生比较已通过。本期支持的 8-bit 窄子集另用固定独立 bilinear 金标准逐像素验收。
