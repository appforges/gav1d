# 私有 gav1d 最小补丁镜像

- 上游固定 gen2brain/gav1d v0.2.7，提交 0d06c9dc16a45726fec53e2f5f63cf1ce61d6da9；保留 COPYING、PATENTS 和历史。
- 只维护 PictureToNRGBA 共享颜色入口、局部 DisableScratchPooling 与机械模块路径调整。不要重写 codec 或改默认行为。
- V05T仅PictureToNRGBA的R/B乘加改为显式float64 FMA再转float32，修正255×255 limited I420单像素49→50；普通Decode继续原路径/算法/SIMD。见PATCHES.md及冻结原失败/独立参考，不推广其他位深/格式或所有C构建。
- 使用 main 主干；不建开发分支或 worktree，不覆盖其他修改。双 Go 1.27.3/1.26.10 race、vet、build，另验 noasm。
- GitHub Actions 暂停，工作流以 .disabled 保留。凭证、构建输出不提交。
- API 输入只读借用，输出独立；Flush、Release、取消不代表硬 RSS/CPU 保证，不强制 GC 或修改全局 GOMAXPROCS。
- SDK 消费前先在 Boundlet 独立开发模块验证；本仓库发布不授权生产 AVIF 或上游 PR。
