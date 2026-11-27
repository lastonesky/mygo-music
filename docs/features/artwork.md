# 列表封面的连续显示

歌曲列表小封面在共享的 64 MiB 位图预算内优先保留最近三个列表页的最后一屏：每页最多 30 张，总保留额度最多 8 MiB。滚动会替换当前页的可见区；重复封面只计算一次。达到硬容量上限时，先回收最旧页面的保留条目。播放封面、大图和模糊背景不使用这项列表额度。

列表位图就绪后立即显示，包括磁盘和本地内嵌封面恢复。读取和解码仍在后台进行；首次访问或已被淘汰的封面允许短暂占位。

歌单刷新在 `p1.music.126.net` 与 `p2.music.126.net` 之间切换同一公开 HTTPS 图片时，显示层在新地址加载期间复用已解码的位图。复用要求相同路径、仅含规范尺寸的查询且没有 Cookie；其他域名、端口、用户信息、版本或签名查询不参与。新地址继续独立请求、解码及验证，磁盘缓存键不合并。明确撤销的新地址仍显示缺失状态。

回归入口：`internal/app/artwork_list_test.go`、`list_cover_test.go`、`list_cover_io_test.go`、`artwork_test.go` 和 `internal/netease/image_cache_test.go`。`NM_FRAMES` 可输出明暗主题、常规和最小窗口的软件渲染结果。`artCache.stats` 的命中、加载、淘汰与占位计数按 UI 构建调用统计，并非独立显示帧数。

移植来源：[Nyago d1f7bb0](https://github.com/DebuNeko233/nyago-music/commit/d1f7bb0c4ac3e81aef58bc4af363676026acee73)、[Nyago bcd22dc](https://github.com/DebuNeko233/nyago-music/commit/bcd22dcc3634c2cb5e4fbe3458bf6c56649ebae7)。
