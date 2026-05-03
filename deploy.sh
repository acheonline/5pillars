#!/bin/bash

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

load_config() {
    local config_file="deploy-config.env"

    log_info "Загрузка конфигурации из $config_file"
    unset DOCKER_REPO DOCKER_PASSWORD REMOTE_USER REMOTE_HOST REMOTE_PASSWORD IMAGE_NAME
    while IFS='=' read -r key value || [ -n "$key" ]; do
        if [[ $key =~ ^# ]] || [[ -z "$key" ]]; then
            continue
        fi

        key=$(echo "$key" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')
        value=$(echo "$value" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//; s/^"//; s/"$//; s/^'"'"'//; s/'"'"'$//')

        export "$key"="$value"

        if [[ "$key" == "DOCKER_PASSWORD" ]] || [[ "$key" == "REMOTE_PASSWORD" ]]; then
            log_info "  $key=********"
        else
            log_info "  $key=$value"
        fi
    done < "$config_file"

    local required_vars=("DOCKER_REPO" "DOCKER_PASSWORD" "REMOTE_USER" "REMOTE_HOST" "REMOTE_PASSWORD" "IMAGE_NAME")
    for var in "${required_vars[@]}"; do
        if [ -z "${!var}" ]; then
            log_error "Переменная $var не задана в конфигурации!"
        fi
    done

    if [ -z "$REMOTE_PATH" ]; then
        REMOTE_PATH="/app/$IMAGE_NAME"
        log_info "  REMOTE_PATH не задан, использую по умолчанию: $REMOTE_PATH"
    fi

    log_success "Конфигурация загружена"
}

check_dependencies() {
    log_info "Проверка зависимостей..."

    if ! command -v docker &> /dev/null; then
        log_error "Docker не установлен!"
    fi

    if ! command -v sshpass &> /dev/null; then
        log_error "sshpass не установлен! Установите его: sudo apt-get install sshpass (Ubuntu/Debian) или brew install hudochenkov/sshpass/sshpass (MacOS)"
    fi

    if ! command -v ssh &> /dev/null; then
        log_error "SSH клиент не установлен!"
    fi

    log_success "Все зависимости проверены"
}

get_build_datetime() {
    BUILD_DATE=$(date +%Y%m%d-%H%M%S)
    BUILD_TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    echo "$BUILD_DATE"
}

build_image() {
    local build_date=$(get_build_datetime)

    local tag_with_date="${IMAGE_NAME}:${build_date}"
    local latest_tag="${IMAGE_NAME}:latest"
    local remote_tag="${DOCKER_REPO}/${IMAGE_NAME}:latest"

    log_info "Начинаю сборку образа..."
    log_info "Название образа: $IMAGE_NAME"
    log_info "Версия: $build_date"
    log_info "Для деплоя: $remote_tag"

    docker build \
        -t "${tag_with_date}" \
        -t "${latest_tag}" \
        --label "build.date=${build_date}" \
        --label "build.timestamp=${BUILD_TIMESTAMP}" \
        --label "maintainer=${DOCKER_REPO}" \
        --progress=plain \
        .

    if [ $? -eq 0 ]; then
        log_success "Образ успешно собран:"
        echo "  - ${tag_with_date} (локальный с версией)"
        echo "  - ${latest_tag} (локальный latest)"

        docker tag "${latest_tag}" "${remote_tag}"
        log_info "Добавлен тег для Docker Hub: ${remote_tag}"

        export IMAGE_TAG="$tag_with_date"
        export IMAGE_REMOTE_TAG="$remote_tag"
    else
        log_error "Ошибка при сборке образа!"
    fi
}

push_image() {
    log_info "Публикация образа в Docker Hub..."

    echo "$DOCKER_PASSWORD" | docker login -u "$DOCKER_REPO" --password-stdin

    docker push "${IMAGE_REMOTE_TAG}"

    if [ $? -eq 0 ]; then
        log_success "Образ успешно опубликован!"
        log_info "Ссылка на образ: https://hub.docker.com/r/${DOCKER_REPO}/${IMAGE_NAME}"

        docker logout
        log_info "Выход из Docker Hub выполнен"
    else
        log_error "Ошибка при публикации образа!"
    fi
}

clean_local_images() {
    log_info "Очистка локальных Docker образов..."

    if [ -n "$IMAGE_TAG" ]; then
        docker rmi "${IMAGE_TAG}" 2>/dev/null || true
        log_info "Удален локальный образ: ${IMAGE_TAG}"
    fi

    local local_latest="${IMAGE_NAME}:latest"
    docker rmi "${local_latest}" 2>/dev/null || true
    log_info "Удален локальный образ: ${local_latest}"

    if [ -n "$IMAGE_REMOTE_TAG" ]; then
        docker rmi "${IMAGE_REMOTE_TAG}" 2>/dev/null || true
        log_info "Удалена локальная копия: ${IMAGE_REMOTE_TAG}"
    fi

    docker image prune -f 2>/dev/null || true
    log_success "Локальная очистка завершена"
}

update_compose_file() {
    log_info "Обновление docker-compose.yml..."

    if [ ! -f "docker-compose.yml" ]; then
        log_error "Файл docker-compose.yml не найден! Создайте его вручную."
    fi

    local new_image="${DOCKER_REPO}/${IMAGE_NAME}:latest"

    log_info "Обновляю image в docker-compose.yml на: $new_image"

    local original_content=$(cat docker-compose.yml)

    local updated_content=$(echo "$original_content" | \
        sed "s|image:.*\"${IMAGE_NAME}\".*|image: \"$new_image\"|g" | \
        sed "s|image:.*${IMAGE_NAME}:.*|image: $new_image|g" | \
        sed "s|image:.*${IMAGE_NAME}\$|image: $new_image|g")

    echo "$updated_content" > docker-compose.yml

    echo ""
    log_info "Было:"
    echo "$original_content" | grep -i "image:"

    log_info "Стало:"
    grep -i "image:" docker-compose.yml

    log_success "docker-compose.yml обновлен"
}

run_remote_cmd() {
    # Полностью очищаем вывод от ANSI-кодов и экранируем специальные символы
    sshpass -p "$REMOTE_PASSWORD" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "${REMOTE_USER}@${REMOTE_HOST}" "$1" 2>&1 | \
        sed 's/\x1b\[[0-9;]*[a-zA-Z]//g' | \
        sed 's/\x1b\[[0-9;]*m//g' | \
        sed 's/\x1b(B//g' | \
        sed 's/\x1b\[?[0-9;]*[hl]//g' | \
        grep -v 'bash: .* command not found' 2>/dev/null || true
    return 0
}

copy_to_remote() {
    sshpass -p "$REMOTE_PASSWORD" scp -o StrictHostKeyChecking=no "$1" "${REMOTE_USER}@${REMOTE_HOST}:$2"
}

install_docker_compose() {
    log_info "Установка Docker Compose на удаленном сервере..."

    # Определяем архитектуру
    local arch=$(run_remote_cmd "uname -m")
    local compose_version="v2.24.0"

    case "$arch" in
        x86_64)
            local compose_url="https://github.com/docker/compose/releases/download/${compose_version}/docker-compose-linux-x86_64"
            ;;
        aarch64)
            local compose_url="https://github.com/docker/compose/releases/download/${compose_version}/docker-compose-linux-aarch64"
            ;;
        *)
            log_error "Неподдерживаемая архитектура: $arch"
            ;;
    esac

    log_info "Загрузка Docker Compose ${compose_version} для архитектуры ${arch}..."
    run_remote_cmd "sudo curl -L ${compose_url} -o /usr/local/bin/docker-compose"
    run_remote_cmd "sudo chmod +x /usr/local/bin/docker-compose"

    # Проверяем установку
    if run_remote_cmd "docker-compose --version" > /dev/null 2>&1; then
        log_success "Docker Compose успешно установлен"
        run_remote_cmd "docker-compose --version"
    else
        log_error "Не удалось установить Docker Compose"
    fi
}

check_docker_compose_cmd() {
    log_info "Проверка команды Docker Compose на удаленном сервере..."

    # Проверяем наличие docker-compose (старая версия)
    if run_remote_cmd "command -v docker-compose &> /dev/null"; then
        echo "docker-compose"
        return 0
    fi

    # Проверяем наличие docker compose (новая версия plugin)
    if run_remote_cmd "docker compose version &> /dev/null"; then
        echo "docker compose"
        return 0
    fi

    # Если ничего не найдено, устанавливаем
    log_warning "Docker Compose не найден на удаленном сервере"
    install_docker_compose

    # После установки проверяем еще раз
    if run_remote_cmd "command -v docker-compose &> /dev/null"; then
        echo "docker-compose"
        return 0
    else
        log_error "Docker Compose не установлен на удаленном сервере!"
    fi
}

run_on_remote() {
    log_info "Запуск Docker Compose на удаленном сервере..."

    # Проверяем SSH подключение
    log_info "Проверка SSH подключения к серверу..."
    if ! run_remote_cmd "echo 'OK'" 2>/dev/null | grep -q "OK"; then
        log_error "Не удалось подключиться к серверу по SSH. Проверьте логин и пароль."
    fi
    log_success "SSH подключение успешно"

    # Определяем команду Docker Compose
    local docker_compose_cmd=$(check_docker_compose_cmd)
    log_info "Использую команду: $docker_compose_cmd"

    # Создаем директорию
    log_info "Создание директории ${REMOTE_PATH} на сервере..."
    run_remote_cmd "sudo mkdir -p ${REMOTE_PATH} && sudo chown -R ${REMOTE_USER}:${REMOTE_USER} ${REMOTE_PATH}" || \
        log_error "Не удалось создать директорию ${REMOTE_PATH}"
    log_success "Директория создана"

    # Логин в Docker Hub
    log_info "Логин в Docker Hub на удаленном сервере..."
    run_remote_cmd "echo '$DOCKER_PASSWORD' | docker login -u '$DOCKER_REPO' --password-stdin" 2>/dev/null | grep -v "Login Succeeded" || true
    log_success "Логин выполнен"

    # Копируем файлы
    log_info "Копирование docker-compose.yml на сервер..."
    copy_to_remote docker-compose.yml "${REMOTE_PATH}/"
    log_success "docker-compose.yml скопирован"

    if [ -f ".env" ]; then
        log_info "Копирование .env на сервер..."
        copy_to_remote .env "${REMOTE_PATH}/"
        log_success ".env скопирован"
    else
        log_warning "Файл .env не найден, копирование пропущено"
    fi

    # Останавливаем старые контейнеры
    log_info "Остановка старых контейнеров (если есть)..."
    run_remote_cmd "cd ${REMOTE_PATH} && ${docker_compose_cmd} down 2>/dev/null || true"

    # Скачиваем образ
    log_info "Скачивание образа на удаленном сервере..."
    run_remote_cmd "docker pull ${DOCKER_REPO}/${IMAGE_NAME}:latest"
    log_success "Образ скачан"

    # Запускаем контейнеры
    log_info "Запуск контейнеров..."
    if run_remote_cmd "cd ${REMOTE_PATH} && ${docker_compose_cmd} up -d"; then
        log_success "Docker Compose успешно запущен на удаленном сервере"
    else
        log_error "Ошибка при запуске Docker Compose на удаленном сервере"
    fi

    # Выход из Docker Hub
    log_info "Выход из Docker Hub на удаленном сервере..."
    run_remote_cmd "docker logout" 2>/dev/null || true
}

check_remote_status() {
    log_info "Проверка статуса на удаленном сервере..."

    local docker_compose_cmd=$(check_docker_compose_cmd 2>/dev/null)

    echo -e "\n${BLUE}--- СТАТУС КОНТЕЙНЕРОВ ---${NC}"
    run_remote_cmd "cd ${REMOTE_PATH} && ${docker_compose_cmd} ps 2>&1 | grep -v 'bash: .* command not found'"

    echo -e "\n${BLUE}--- ПОСЛЕДНИЕ ЛОГИ ---${NC}"
    run_remote_cmd "cd ${REMOTE_PATH} && ${docker_compose_cmd} logs --tail=10 2>&1 | grep -v 'bash: .* command not found'"

    echo -e "\n${BLUE}--- ИНФОРМАЦИЯ О СИСТЕМЕ ---${NC}"
    run_remote_cmd "echo 'Загрузка CPU:' && uptime && echo -e '\nСвободная память:' && free -h 2>&1 | grep -v 'bash: .* command not found'"

    echo -e "\n${GREEN}Приложение доступно: http://${REMOTE_HOST}:8080${NC}"
    echo -e "${GREEN}Telegram бот: @five_pillars_bot${NC}"
}

main() {
    echo -e "${BLUE}╔══════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║   ДЕПЛОЙ DOCKER ПРИЛОЖЕНИЯ ${IMAGE_NAME:-GOALS}     ║${NC}"
    echo -e "${BLUE}╚══════════════════════════════════════╝${NC}"

    load_config
    check_dependencies
    build_image
    push_image
    update_compose_file
    run_on_remote
    check_remote_status
    clean_local_images

    echo -e "\n${GREEN}════════════════════════════════════════${NC}"
    echo -e "${GREEN}           ДЕПЛОЙ ЗАВЕРШЕН!             ${NC}"
    echo -e "${GREEN}════════════════════════════════════════${NC}"
    echo -e "Образ: ${IMAGE_REMOTE_TAG}"
    echo -e "Сервер: ${REMOTE_USER}@${REMOTE_HOST}"
    echo -e "Путь: ${REMOTE_PATH}"
    echo -e "${GREEN}════════════════════════════════════════${NC}"
}

main "$@"