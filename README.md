## .env example

# name for service (used as log prefix)
SERVICE=mqtt-bridge

# path to config.yml (contains log settings)
CONFIG_FILE=/conf/config.yml

# path to forward.yml (contains MQTT bridge rules and device filters)
FORWARD_CONF_FILE=/conf/forward.yml

# LOG_LEVEL=debug|info|warn|error|fatal
LOG_LEVEL=debug

# log output target: os | fluentd
LOG_TARGET=os