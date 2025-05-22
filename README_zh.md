[![Go Report Card](https://goreportcard.com/badge/github.com/ochinchina/supervisord)](https://goreportcard.com/report/github.com/ochinchina/supervisord)

# 为什么需要这个项目？

Python 脚本 supervisord 是一个强大的进程管理工具，被很多人使用。我也很喜欢 supervisord。

但是这个工具需要在目标系统中安装完整的 Python 环境。在某些情况下，比如在 Docker 环境中，Python 对我们来说太大了。

这个项目用 Go 语言重新实现了 supervisord。编译后的 supervisord 非常适合没有安装 Python 的环境。

# 构建 supervisord

在编译 supervisord 之前，请确保你的环境中已安装 Go 语言 1.11+ 版本。

要为 **Linux** 编译 supervisord，请运行以下命令：

1. go generate
2. GOOS=linux go build -tags release -a -ldflags "-linkmode external -extldflags -static" -o supervisord

# 运行 supervisord

生成 supervisord 二进制文件后，创建一个 supervisord 配置文件并按如下方式启动 supervisord：

```Shell
$ cat supervisor.conf
[program:test]
command = /your/program args
$ supervisord -c supervisor.conf
```

请注意，配置文件位置按以下顺序自动检测：

1. $CWD/supervisord.conf
2. $CWD/etc/supervisord.conf
3. /etc/supervisord.conf
4. /etc/supervisor/supervisord.conf (自 Supervisor 3.3.0 起)
5. ../etc/supervisord.conf (相对于可执行文件)
6. ../supervisord.conf (相对于可执行文件)

# 以守护进程方式运行并启用 Web UI

在配置文件中添加 inet 接口：

```ini
[inet_http_server]
port=127.0.0.1:9001
```

然后运行

```shell
$ supervisord -c supervisor.conf -d
```

为了管理守护进程，你可以使用 `supervisord ctl` 子命令，可用的子命令有：`status`、`start`、`stop`、`shutdown`、`reload`。

```shell
$ supervisord ctl status
$ supervisord ctl status program-1 program-2...
$ supervisord ctl status group:*
$ supervisord ctl stop program-1 program-2...
$ supervisord ctl stop group:*
$ supervisord ctl stop all
$ supervisord ctl start program-1 program-2...
$ supervisord ctl start group:*
$ supervisord ctl start all
$ supervisord ctl shutdown
$ supervisord ctl reload
$ supervisord ctl signal <signal_name> <process_name> <process_name> ...
$ supervisord ctl signal all
$ supervisord ctl pid <process_name>
$ supervisord ctl fg <process_name>
```

请注意，`supervisor ctl` 子命令只有在 [inet_http_server] 中启用了 http 服务器，并且正确设置了 **serverurl** 时才能正常工作。目前不支持 Unix 域套接字用于此目的。

Serverurl 参数按以下顺序检测：

- 检查是否存在 -s 或 --serverurl 选项，如果存在则使用该 URL
- 检查是否存在 -c 选项，以及 "supervisorctl" 部分中是否存在 "serverurl"，如果存在则使用 "supervisorctl" 部分中的 "serverurl"
- 检查自动检测到的 supervisord.conf 文件位置中是否定义了 "supervisorctl" 部分中的 "serverurl"，如果存在则使用找到的值
- 使用 http://localhost:9001

# 检查版本

"version" 命令将显示当前 supervisord 二进制文件的版本。

```shell
$ supervisord version
```

# 支持的功能

## Http 服务器

Http 服务器可以通过 Unix 域套接字和 TCP 工作。基本认证是可选的，也受支持。

Unix 域套接字设置在 "unix_http_server" 部分。
TCP http 服务器设置在 "inet_http_server" 部分。

如果在配置文件中没有设置 "inet_http_server" 和 "unix_http_server"，则不会启动 http 服务器。

## Supervisord 守护进程设置

以下参数在 "supervisord" 部分配置：

- **logfile**。supervisord 自身的日志存放位置。
- **logfile_maxbytes**。当日志文件超过此长度时进行轮转。
- **logfile_backups**。保留的轮转日志文件数量。
- **loglevel**。日志详细程度，可以是 trace、debug、info、warning、error、fatal 和 panic（根据用于此功能的模块文档）。默认为 info。
- **pidfile**。包含当前 supervisord 实例进程 ID 的文件的完整路径。
- **minfds**。在 supervisord 启动时至少保留此数量的文件描述符。（Rlimit nofiles）。
- **minprocs**。在 supervisord 启动时至少保留此数量的进程资源。（Rlimit noproc）。
- **identifier**。此 supervisord 实例的标识符。如果在同一台机器的同一命名空间中运行多个 supervisord，则需要此参数。

## 被监督程序设置

被监督程序设置在 [program:programName] 部分配置，包括以下选项：

- **command**。要监督的命令。可以给出可执行文件的完整路径，也可以通过 PATH 变量计算。命令行参数也应该在此字符串中提供。
- **process_name**。进程名称
- **numprocs**。进程数量
- **numprocs_start**。？？
- **autostart**。是否在 supervisord 启动时运行被监督的命令？默认为 **true**。
- **startsecs**。程序在启动后需要保持运行的总秒数，以认为启动成功（将进程从 STARTING 状态移动到 RUNNING 状态）。设置为 0 表示程序不需要保持运行任何特定时间。
- **startretries**。supervisord 在尝试启动程序时允许的连续失败尝试次数，超过此次数后将放弃并将进程置于 FATAL 状态。有关 FATAL 状态的说明，请参见进程状态。
- **autorestart**。如果被监督的命令死亡，是否自动重新运行。
- **exitcodes**。与 autorestart 一起使用的程序的"预期"退出代码列表。如果 autorestart 参数设置为 unexpected，并且进程以除 supervisor stop 请求结果之外的任何方式退出，如果进程以未在此列表中定义的退出代码退出，supervisord 将重新启动进程。
- **stopsignal**。发送给命令以优雅停止的信号。如果配置了多个 stopsignal，在停止程序时，supervisor 将按顺序向程序发送信号，间隔为 "stopwaitsecs"。如果程序在所有信号发送后仍未退出，supervisord 将终止程序。
- **stopwaitsecs**。在向被监督的命令发送 SIGKILL 以使其不优雅地停止之前等待的时间。
- **stdout_logfile**。被监督命令的 STDOUT 应该重定向到哪里。（特定值在本文档后面描述）。
- **stdout_logfile_maxbytes**。超过此大小后将轮转日志。
- **stdout_logfile_backups**。保留的轮转日志文件数量。
- **redirect_stderr**。是否将 STDERR 重定向到 STDOUT。
- **stderr_logfile**。被监督命令的 STDERR 应该重定向到哪里。（特定值在本文档后面描述）。
- **stderr_logfile_maxbytes**。超过此大小后将轮转日志。
- **stderr_logfile_backups**。保留的轮转日志文件数量。
- **environment**。要传递给被监督程序的 VARIABLE=value 列表。它的优先级高于 `envFiles`。
- **envFiles**。要加载并传递给被监督程序的 .env 文件列表。
- **priority**。程序在启动和关闭顺序中的相对优先级
- **user**。在执行被监督命令之前切换到该 USER 或 USER:GROUP。
- **directory**。跳转到此路径并在那里执行被监督命令。
- **stopasgroup**。在停止此程序所在的程序组时也停止此程序。
- **killasgroup**。在停止此程序所在的程序组时也终止此程序。
- **restartpause**。在停止被监督程序后等待（至少）这么多秒再重新启动它。
- **restart_when_binary_changed**。布尔值（false 或 true），用于控制当被监督命令的可执行二进制文件更改时是否应该重新启动它。默认为 false。
- **restart_cmd_when_binary_changed**。如果程序二进制文件本身发生变化，用于重新启动程序的命令。
- **restart_signal_when_binary_changed**。如果程序二进制文件发生变化，用于重新启动程序的信号。
- **restart_directory_monitor**。用于重新启动目的的监控路径。
- **restart_file_pattern**。如果在 restart_directory_monitor 下的文件发生变化且文件名匹配此模式，将重新启动被监督命令。
- **restart_cmd_when_file_changed**。如果在 **restart_directory_monitor** 下使用模式 **restart_file_pattern** 监控的任何文件发生变化，用于重新启动程序的命令。
- **restart_signal_when_file_changed**。如果在 **restart_directory_monitor** 下使用模式 **restart_file_pattern** 监控的任何文件发生变化，将发送给程序（如 Nginx）用于重新启动的信号。
- **depends_on**。定义被监督命令的启动依赖关系。如果程序 A 依赖于程序 B、C，则程序 B、C 将在程序 A 之前启动。示例：

```ini
[program:A]
depends_on = B, C

[program:B]
...
[program:C]
...
```

## 为所有被监督程序设置默认参数

所有被监督程序共有的相同参数可以在 "program-default" 部分定义一次，并在所有其他程序部分中省略。

在下面的示例中，VAR1 和 VAR2 环境变量适用于 test1 和 test2 被监督程序：

```ini
[program-default]
environment=VAR1="value1",VAR2="value2"
envFiles=global.env,prod.env

[program:test1]
...

[program:test2]
...

```

## 组

支持 "group" 部分，你可以设置 "programs" 项

## 事件

部分支持 Supervisord 3.x 定义的事件。现在支持以下事件：

- 所有进程状态相关事件
- 进程通信事件
- 远程通信事件
- tick 相关事件
- 进程日志相关事件

## 日志

Supervisord 可以将被监督程序的 stdout 和 stderr（字段 stdout_logfile、stderr_logfile）重定向到：

- **/dev/null**。忽略日志 - 发送到 /dev/null。
- **/dev/stdout**。将日志写入 STDOUT。
- **/dev/stderr**。将日志写入 STDERR。
- **syslog**。将日志发送到本地 syslog 服务。
- **syslog @[protocol:]host[:port]**。将日志事件发送到远程 syslog 服务器。协议必须是 "tcp" 或 "udp"，如果缺失，则假定为 "udp"。如果端口缺失，对于 "udp" 协议，默认为 514，对于 "tcp" 协议，值为 6514。
- **file name**。将日志写入指定文件。

可以为 stdout_logfile 和 stderr_logfile 配置多个日志文件，使用 ',' 作为分隔符。例如：

```ini
stdout_logfile = test.log, /dev/stdout
```

### syslog 设置

如果将日志写入 syslog，可以设置以下附加参数：
```ini
syslog_facility=local0
syslog_tag=test
syslog_stdout_priority=info
syslog_stderr_priority=err
```
- **syslog_facility**，可以是以下之一（不区分大小写）：KERNEL、USER、MAIL、DAEMON、AUTH、SYSLOG、LPR、NEWS、UUCP、CRON、AUTHPRIV、FTP、LOCAL0~LOCAL7
- **syslog_stdout_priority**，可以是以下之一（不区分大小写）：EMERG、ALERT、CRIT、ERR、WARN、NOTICE、INFO、DEBUG
- **syslog_stderr_priority**，可以是以下之一（不区分大小写）：EMERG、ALERT、CRIT、ERR、WARN、NOTICE、INFO、DEBUG

# Web GUI

Supervisord 有内置的 Web GUI：你可以从 GUI 中启动、停止和检查程序状态。下图显示了默认的 Web GUI：

![alt text](https://github.com/ochinchina/supervisord/blob/master/go_supervisord_gui.png)

请注意，要查看|使用 Web GUI，你应该在 /etc/supervisord.conf 中配置它，包括 [inet_http_server]（和|或 [unix_http_server]，如果你更喜欢 Unix 域套接字）和 [supervisorctl]：

```ini
[inet_http_server]
port=127.0.0.1:9001
;username=test1
;password=thepassword

[supervisorctl]
serverurl=http://127.0.0.1:9001
```

# 在 Docker 容器中使用

supervisord 在 Docker 镜像中编译，可以直接在另一个镜像中使用，从 Docker Hub 版本。

```Dockerfile
FROM debian:latest
COPY --from=ochinchina/supervisord:latest /usr/local/bin/supervisord /usr/local/bin/supervisord
CMD ["/usr/local/bin/supervisord"]
```

# 与 Prometheus 集成

Prometheus node exporter 支持的 supervisord 指标现在已集成到 supervisor 中。因此，不需要部署额外的 node_exporter 来收集 supervisord 指标。要收集指标，必须在 "inet_http_server" 部分配置 port 参数，指标服务器在 supervisor http 服务器的 /metrics 路径上启动。

例如，如果 "inet_http_server" 中的 port 参数是 "127.0.0.1:9001"，那么应该通过 URL "http://127.0.0.1:9001/metrics" 访问指标服务器

# 注册服务

在操作系统启动后自动启动 supervisord。查看 [kardianos/service](https://github.com/kardianos/service) 支持的平台。

```Shell
# 安装
sudo supervisord service install -c full_path_to_conf_file
# 卸载
sudo supervisord service uninstall
# 启动
supervisord service start
# 停止
supervisord service stop
``` 

# 使用Nacos配置

Supervisord支持从Nacos配置中心获取配置，这使得在分布式环境中管理配置变得更加简单。

## 命令行参数

使用以下命令行参数来指定Nacos配置：

```shell
supervisord --nacos-server=127.0.0.1:8848 --nacos-dataid=supervisord.conf --nacos-namespace=public --nacos-group=DEFAULT_GROUP --nacos-username=nacos --nacos-password=nacos --nacos-not-use-cache
```

参数说明：
- `--nacos-server`：Nacos服务器地址，格式为`IP:PORT`
- `--nacos-dataid`：Nacos配置ID
- `--nacos-namespace`：Nacos命名空间ID（可选）
- `--nacos-group`：Nacos配置分组，默认为`DEFAULT_GROUP`（可选）
- `--nacos-username`：Nacos用户名（可选）
- `--nacos-password`：Nacos密码（可选）
- `--nacos-not-use-cache`：不使用本地缓存，直接从Nacos服务器获取最新配置（可选）

## Web界面配置

也可以通过Web界面配置Nacos：

1. 启动supervisord（使用本地配置文件）
2. 访问Web界面：http://localhost:9001/
3. 点击"Nacos配置"按钮
4. 填写Nacos服务器地址、配置ID等信息
5. 勾选"不使用缓存"选项（如果需要）
6. 点击"保存配置"按钮
7. 重启supervisord以使用Nacos配置

## 缓存控制

`--nacos-not-use-cache`参数（或Web界面中的"不使用缓存"选项）可以控制Nacos客户端是否使用本地缓存：

- 当启用此选项时，Nacos客户端将不会在启动时加载本地缓存的配置，而是直接从服务器获取最新配置。这对于确保始终使用最新配置非常有用，特别是在配置频繁变更的环境中。

- 当不启用此选项时，Nacos客户端会在启动时尝试加载本地缓存的配置，这可以提高启动速度，并在Nacos服务器暂时不可用时提供一定的容错能力。

## 配置管理

使用Nacos配置时，可以通过Web界面直接添加、修改、删除和复制程序，这些操作会直接修改Nacos中的配置。这使得在分布式环境中管理supervisord配置变得更加简单和一致。
