# FollowITup 部署指南(Windows Server 2022 离线环境)

> 目标服务器:Windows Server 2022 · 已装 IIS(不使用)· C 盘系统盘 / D 盘数据盘 · **服务器不能访问互联网**
> 决策(2026-08-17 确认):直接端口 **8081** 监听(不碰 IIS)· NSSM 注册服务 · 本地账号认证(登录名=用户邮箱)· 邮件发往**内网 SMTP 服务器**

## 1. 总体架构

```
浏览器(内网) ── http://<服务器IP>:8081 ──> followitup.exe(Windows 服务, NSSM)
                                              ├─ SQLite 数据库 D:\FollowITup\data\
                                              └─ SMTP(内网邮件服务器)← 到期提醒/密码邮件
```

- 单 exe 零运行时依赖(无 Java / Node / IIS 依赖)——**离线部署不受影响**
- IIS 已装但不需要:它占 80/443,FollowITup 用 8081 不冲突
- 唯一受离线影响的功能:SMTP 必须内网可达(已有内网 SMTP,可用)

## 2. 目录规划(C 系统盘 / D 数据盘)

```
D:\FollowITup\                    ← 程序与数据同盘,备份一起拷
├── followitup.exe                ← 程序(含内嵌前端)
├── config.yaml                   ← 配置(见 §4)
├── nssm.exe                      ← 服务管理工具
├── logs\                         ← NSSM 日志输出
├── data\                         ← SQLite 数据库(单文件 + WAL)
└── backup\                       ← 每日备份(见 §8)
```

## 3. 准备 exe 与工具(在开发机构建,经 U 盘/内网共享拷贝)

```bash
# 1. 构建前端并嵌入(开发机)
cd frontend && npm run build
rm -rf backend/cmd/server/frontend-dist && cp -r frontend/dist backend/cmd/server/frontend-dist
cd backend && go build -o followitup.exe ./cmd/server/

# 2. 获取 nssm(https://nssm.cc,离线可随 exe 一起拷贝)
# 3. 两个文件拷到服务器 D:\FollowITup\
```

## 4. 配置 config.yaml

```yaml
server:
  port: 8081                    # ← 用户指定端口
  data_dir: "D:/FollowITup/data" # ← 绝对路径(相对路径会随工作目录漂移丢库)

fiscal:
  year_start_month: 4

auth:
  jwt_secret: "<随机串>"         # ← 必改!见下
  session_hours: 8
  bcrypt_cost: 12

ldap:
  enabled: false                 # 本地账号认证,不启用 LDAP
```

**jwt_secret 生成**(开发机生成,写进 config 再拷贝):
```bash
python -c "import secrets; print(secrets.token_hex(32))"
```

**SMTP 不用写在 config.yaml**——在 Web 界面配置:登录管理员 → 系统设置 → SMTP 配置(服务器地址、端口、认证、发件人,内网 SMTP),有"测试发送"按钮验证。邮件功能(到期提醒、密码重置/临时密码)依赖此配置。

## 5. 首次运行验证(前台)

```bat
cd /d D:\FollowITup
followitup.exe -config config.yaml
```

- 日志出现 `FollowITup v1.8.10 启动于 http://localhost:8081` 即成功
- 自动创建数据库与管理员:`admin@followitup.local` / `admin123`
- **首次登录后立即修改管理员密码**(用户管理页)
- 验证后 Ctrl+C 停止,注册服务

## 6. 注册为 Windows 服务(NSSM)

```bat
cd /d D:\FollowITup

:: 安装服务(参数:服务名、程序、参数)
nssm install FollowITup "D:\FollowITup\followitup.exe" "-config D:\FollowITup\config.yaml"

:: 工作目录(关键:数据库绝对路径已配置,目录正确性双保险)
nssm set FollowITup AppDirectory "D:\FollowITup"

:: 日志落盘 + 滚动(10MB/份)
nssm set FollowITup AppStdout "D:\FollowITup\logs\followitup.log"
nssm set FollowITup AppStderr "D:\FollowITup\logs\followitup-error.log"
nssm set FollowITup AppRotateFiles 1
nssm set FollowITup AppRotateOnline 1
nssm set FollowITup AppRotateBytes 10485760

:: 开机自启 + 启动
nssm set FollowITup Start SERVICE_AUTO_START
nssm start FollowITup
```

常用管理命令:

```bat
nssm restart FollowITup    :: 重启(升级后)
nssm stop FollowITup       :: 停止
nssm edit FollowITup       :: 图形界面改配置
sc query FollowITup        :: 查看状态
```

## 7. 防火墙放行 8081

```bat
netsh advfirewall firewall add rule name="FollowITup 8081" dir=in action=allow protocol=TCP localport=8081
```

验证:内网另一台机器浏览器访问 `http://<服务器IP>:8081`

## 8. 备份策略(SQLite 单文件,每日停服备份)

`D:\FollowITup\backup.ps1`:

```powershell
$stamp = Get-Date -Format "yyyyMMdd-HHmm"
Stop-Service FollowITup
Start-Sleep 2
Copy-Item "D:\FollowITup\data" "D:\FollowITup\backup\data-$stamp" -Recurse
Start-Service FollowITup
# 保留最近 30 份
Get-ChildItem "D:\FollowITup\backup" -Directory | Sort-Object Name -Descending | Select-Object -Skip 30 | Remove-Item -Recurse -Force
```

注册每日任务(凌晨 2 点):

```bat
schtasks /create /tn "FollowITup备份" /tr "powershell -ExecutionPolicy Bypass -File D:\FollowITup\backup.ps1" /sc daily /st 02:00
```

> 说明:SQLite 使用 WAL 模式,备份前停服是最稳妥的方式(短时停服,凌晨无感)。若不能停服,可改用 `VACUUM INTO` 在线备份(需在服务内实现,当前版本未提供)。

## 9. 升级流程(后续版本)

```bat
nssm stop FollowITup
:: 备份 data 目录(见 §8,升级前必做)
:: 覆盖 D:\FollowITup\followitup.exe(新版本)
nssm start FollowITup
```

- 数据库迁移是版本化的(启动时自动执行,向前兼容),无需手工操作
- 升级后浏览器强刷(Ctrl+F5)避免旧前端缓存

## 10. 常见问题

| 现象 | 排查 |
|------|------|
| 服务启动即失败 | 看 `logs\followitup-error.log`;前台运行复现错误 |
| 浏览器访问不通 | 防火墙规则、端口占用(`netstat -ano | findstr 8081`)、服务状态 |
| 数据库"消失" | config.yaml 的 data_dir 必须是绝对路径(§4) |
| 邮件发不出 | 系统设置 → SMTP → 测试发送;服务器能否连通内网 SMTP 的端口(telnet) |
| 忘记管理员密码 | 停服 → 数据库 `users` 表重置密码哈希(或重建库)→ 启服 |
| 端口被占 | config.yaml 改端口 + 防火墙同步放行 |

## 11. 安全清单(部署后逐项确认)

- [ ] jwt_secret 已改为随机串(§4)
- [ ] 管理员初始密码已修改
- [ ] 防火墙仅放行 8081(内网范围可收紧为指定网段:`remoteip=192.168.0.0/16` 等)
- [ ] SMTP 配置完成(若需邮件)
- [ ] 每日备份任务已注册(§8)
- [ ] 服务设为自动启动(NSSM 已设)
