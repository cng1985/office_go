#!/usr/bin/env bash
# 安装或更新云笺 systemd 服务。重复执行会保留已有配置和数据。
set -euo pipefail

APP_NAME="yunjian"
INSTALL_DIR="/opt/yunjian"
DATA_DIR="/var/lib/yunjian"
SERVICE_USER="yunjian"
UNIT_PATH="/etc/systemd/system/${APP_NAME}.service"

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

if [[ "$(id -u)" -ne 0 ]]; then
  echo "请使用 root 运行安装脚本" >&2
  exit 1
fi

case "$(uname -m)" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "不支持的架构: $(uname -m)" >&2
    exit 1
    ;;
esac

if [[ -x "${SCRIPT_DIR}/yunjian" ]]; then
  SOURCE_BIN="${SCRIPT_DIR}/yunjian"
  SOURCE_CONFIG="${SCRIPT_DIR}/configs"
elif [[ -x "${SCRIPT_DIR}/../release/linux-${ARCH}/yunjian" ]]; then
  SOURCE_BIN="${SCRIPT_DIR}/../release/linux-${ARCH}/yunjian"
  SOURCE_CONFIG="${SCRIPT_DIR}/../release/linux-${ARCH}/configs"
else
  echo "未找到 linux/${ARCH} 二进制。请先运行 scripts/build-platforms.ps1" >&2
  exit 1
fi

if ! id "${SERVICE_USER}" >/dev/null 2>&1; then
  useradd --system --home "${DATA_DIR}" --shell /usr/sbin/nologin "${SERVICE_USER}"
fi

install -d -o "${SERVICE_USER}" -g "${SERVICE_USER}" -m 0750 "${INSTALL_DIR}" "${INSTALL_DIR}/configs" "${DATA_DIR}"
TMP_BIN="${INSTALL_DIR}/yunjian.new"
install -o root -g "${SERVICE_USER}" -m 0750 "${SOURCE_BIN}" "${TMP_BIN}"
if [[ ! -f "${INSTALL_DIR}/configs/application.yaml" ]]; then
  cp -a "${SOURCE_CONFIG}/." "${INSTALL_DIR}/configs/"
  chown -R root:"${SERVICE_USER}" "${INSTALL_DIR}/configs"
  find "${INSTALL_DIR}/configs" -type d -exec chmod 0750 {} \;
  find "${INSTALL_DIR}/configs" -type f -exec chmod 0640 {} \;
fi

cat > "${UNIT_PATH}" <<EOF
[Unit]
Description=Yunjian cloud office
After=network.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/yunjian --profile pro --config-dir ${INSTALL_DIR}/configs --data-dir ${DATA_DIR}
Restart=on-failure
RestartSec=3
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

if systemctl is-active --quiet "${APP_NAME}"; then
  systemctl stop "${APP_NAME}"
  WAS_ACTIVE=1
else
  WAS_ACTIVE=0
fi

mv -f "${TMP_BIN}" "${INSTALL_DIR}/yunjian"
systemctl daemon-reload
if [[ "${WAS_ACTIVE}" -eq 1 ]] || systemctl is-enabled --quiet "${APP_NAME}"; then
  systemctl restart "${APP_NAME}"
else
  systemctl enable --now "${APP_NAME}"
fi

systemctl --no-pager --full status "${APP_NAME}"
echo "查看日志: journalctl -u ${APP_NAME}"
echo "配置目录 ${INSTALL_DIR}/configs 和数据目录 ${DATA_DIR} 在更新时保留。"
echo "若需回退，请恢复上一份二进制到 ${INSTALL_DIR}/yunjian 后执行 systemctl restart ${APP_NAME}。数据库迁移不随二进制自动回滚。"
