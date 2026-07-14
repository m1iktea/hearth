#!/usr/bin/env bash
# 在 PVE 宿主机上创建温度采集专用账号（供 Hearth 通过 SSH 执行 sensors -j）。
# 以 root 执行：bash pve-setup-sensors.sh
# 账号被 sshd ForceCommand 锁死：无论客户端请求什么命令，服务端只会执行
# `sensors -j` 并返回结果，无法获得交互 shell、端口转发或其他任何能力。
set -euo pipefail

SENSOR_USER="hearth-sensors"
SSHD_DROPIN="/etc/ssh/sshd_config.d/${SENSOR_USER}.conf"

if [[ $(id -u) -ne 0 ]]; then
  echo "错误：请以 root 在 PVE 宿主机上执行" >&2
  exit 1
fi

# 密码优先从参数取（便于自动化）；否则交互输入，避免留在 shell history
PASSWORD="${1:-}"
if [[ -z "${PASSWORD}" ]]; then
  read -rsp "为 ${SENSOR_USER} 设置 SSH 密码: " PASSWORD; echo
  read -rsp "再输入一次确认: " PASSWORD2; echo
  [[ "${PASSWORD}" == "${PASSWORD2}" ]] || { echo "错误：两次输入不一致" >&2; exit 1; }
fi
[[ -n "${PASSWORD}" ]] || { echo "错误：密码不能为空" >&2; exit 1; }

echo "==> 安装 lm-sensors"
apt-get update -qq
apt-get install -y -qq lm-sensors

echo "==> 验证温度传感器可读"
if ! sensors -j >/dev/null 2>&1; then
  echo "sensors -j 无输出，尝试自动探测内核模块（sensors-detect --auto）"
  sensors-detect --auto >/dev/null
  sensors -j >/dev/null || { echo "错误：仍读不到温度传感器，请手动检查 sensors 输出" >&2; exit 1; }
fi

echo "==> 创建受限账号 ${SENSOR_USER}"
if ! id "${SENSOR_USER}" &>/dev/null; then
  useradd --create-home --shell /bin/bash "${SENSOR_USER}"
fi
echo "${SENSOR_USER}:${PASSWORD}" | chpasswd

echo "==> 写入 sshd 限制（${SSHD_DROPIN}）"
cat > "${SSHD_DROPIN}" <<EOF
Match User ${SENSOR_USER}
    ForceCommand /usr/bin/sensors -j
    PasswordAuthentication yes
    PermitTTY no
    AllowTcpForwarding no
    AllowAgentForwarding no
    AllowStreamLocalForwarding no
    X11Forwarding no
EOF
sshd -t
systemctl reload sshd 2>/dev/null || systemctl reload ssh

echo "==> 完成。本机自测："
su -s /bin/bash -c "sensors -j | head -c 200" "${SENSOR_USER}" && echo
cat <<EOF

下一步：在部署 Hearth 的机器上验证并填入 deploy/.env
  ssh ${SENSOR_USER}@<PVE的IP>    # 应直接打印温度 JSON 后退出，无法进入 shell

  PVE_SSH_HOST=<PVE的IP>
  PVE_SSH_USER=${SENSOR_USER}
  PVE_SSH_PASSWORD=<刚设置的密码>
EOF
