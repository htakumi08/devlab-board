#!/bin/sh

# devlab-boardのDocker Compose環境をサービス単位で再構築するスクリプト。
#
# 実行方法:
#   ./build.sh
#     frontend、backend、DBを再構築するか3回質問する。
#     yを選んだサービスだけ、コンテナ・image・専用volumeを削除して再作成する。
#
#   ./build.sh --dry-run
#     通常実行と同じ3回の質問を行い、選択された処理内容だけを表示する。
#     Dockerリソースの削除、imageのbuild/pull、コンテナの起動は行わない。
#
#   ./build.sh --yes
#     3回の質問を省略し、frontend、backend、DBをすべて選択する。
#     Compose全体を削除した後、全サービスを再構築して起動する。
#     PostgreSQLのデータvolumeも削除されるため注意すること。
#
#   ./build.sh --yes --dry-run
#     全サービスを選択した場合の処理予定だけを表示する。
#
# 共通仕様:
#   - 選択したimageはキャッシュを使わずに再ビルドする。
#   - Docker全体のbuild cacheは、他プロジェクトへの影響を避けるため削除しない。
#   - 一部サービスだけを選んだ場合、未選択サービスと共有networkは維持する。

set -eu

# Composeプロジェクト名と設定ファイルは環境変数で上書きできる。
PROJECT_NAME="${COMPOSE_PROJECT_NAME:-devlab-board}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"
ASSUME_YES=false
DRY_RUN=false

usage() {
  cat <<'EOF'
使い方: ./build.sh [--yes] [--dry-run]

frontend、backend、DBを対話形式で選択し、選択したサービスだけを再構築します。

オプション:
  --yes      3回の質問を省略して、すべてのサービスを選択します。
  --dry-run  Dockerを変更せず、選択した処理の予定だけを表示します。
  -h, --help このヘルプを表示します。
EOF
}

# コマンドラインオプションを解析する。
for argument in "$@"; do
  case "$argument" in
    --yes)
      ASSUME_YES=true
      ;;
    --dry-run)
      DRY_RUN=true
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      printf 'Unknown argument: %s\n\n' "$argument" >&2
      usage >&2
      exit 2
      ;;
  esac
done

# どのディレクトリから実行しても、リポジトリルートを作業場所にする。
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$SCRIPT_DIR"

# 削除処理を始める前にDockerとCompose設定を検証する。
if ! command -v docker >/dev/null 2>&1; then
  echo "Error: docker command was not found." >&2
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  echo "Error: Docker daemon is not running." >&2
  exit 1
fi

compose() {
  docker compose \
    --project-name "$PROJECT_NAME" \
    --file "$COMPOSE_FILE" \
    "$@"
}

if [ ! -f "$COMPOSE_FILE" ]; then
  printf 'Error: Compose file was not found: %s\n' "$COMPOSE_FILE" >&2
  exit 1
fi

compose config >/dev/null

echo "Docker Compose project: $PROJECT_NAME"
echo "Compose file: $COMPOSE_FILE"
echo

# y/yesを肯定、n/noまたは未入力を否定として扱う。
ask_yes_no() {
  prompt=$1

  while true; do
    printf '%s [y/N]: ' "$prompt"
    if ! IFS= read -r answer; then
      echo
      return 1
    fi

    case "$answer" in
      y | Y | yes | YES)
        return 0
        ;;
      "" | n | N | no | NO)
        return 1
        ;;
      *)
        echo "yesまたはnoで入力してください。"
        ;;
    esac
  done
}

# 通常実行では、処理対象をサービスごとに3回確認する。
SELECT_FRONTEND=false
SELECT_BACKEND=false
SELECT_DB=false

if [ "$ASSUME_YES" = true ]; then
  SELECT_FRONTEND=true
  SELECT_BACKEND=true
  SELECT_DB=true
else
  if ask_yes_no "frontendを再構築しますか？"; then
    SELECT_FRONTEND=true
  fi

  if ask_yes_no "backendを再構築しますか？"; then
    SELECT_BACKEND=true
  fi

  if ask_yes_no "DBを再構築しますか？ PostgreSQLデータは完全に削除されます。"; then
    SELECT_DB=true
  fi
fi

if [ "$SELECT_FRONTEND" = false ] &&
  [ "$SELECT_BACKEND" = false ] &&
  [ "$SELECT_DB" = false ]; then
  echo "No services were selected. Nothing to do."
  exit 0
fi

echo
echo "Selected services:"
echo "  frontend: $SELECT_FRONTEND"
echo "  backend:  $SELECT_BACKEND"
echo "  db:       $SELECT_DB"
echo

if [ "$SELECT_DB" = true ]; then
  echo "WARNING: PostgreSQL data in devlab-board_postgres18-data will be permanently deleted."
fi

echo "Only volumes belonging to selected services will be removed."
echo "The shared Compose network is removed only when all three services are selected."
echo "Global Docker build cache is kept; selected images are rebuilt with --no-cache."

# dry-runでは予定を表示するだけで、以降の変更処理へ進まない。
if [ "$DRY_RUN" = true ]; then
  echo
  echo "[dry-run] Planned operations:"

  if [ "$SELECT_FRONTEND" = true ]; then
    echo "  - remove frontend container, image, and frontend-node-modules volume"
    echo "  - rebuild and start frontend"
  fi

  if [ "$SELECT_BACKEND" = true ]; then
    echo "  - remove backend container, image, and go-pkg-mod volume"
    echo "  - rebuild and start backend"
  fi

  if [ "$SELECT_DB" = true ]; then
    echo "  - remove db container, postgres:18-alpine image, and postgres18-data volume"
    echo "  - pull and start db"
  fi

  if [ "$SELECT_FRONTEND" = true ] &&
    [ "$SELECT_BACKEND" = true ] &&
    [ "$SELECT_DB" = true ]; then
    echo "  - remove and recreate the Compose network"
  fi

  exit 0
fi

# 各サービスが専用で利用する名前付きvolumeを対応付ける。
volume_key_for_service() {
  case "$1" in
    frontend)
      echo "frontend-node-modules"
      ;;
    backend)
      echo "go-pkg-mod"
      ;;
    db)
      echo "postgres18-data"
      ;;
  esac
}

# コンテナやCompose labelから、選択したサービスに関係するimage IDを集める。
service_image_ids() {
  service=$1

  container_ids=$(docker container ls --all --quiet \
    --filter "label=com.docker.compose.project=$PROJECT_NAME" \
    --filter "label=com.docker.compose.service=$service")

  for container_id in $container_ids; do
    docker container inspect --format '{{.Image}}' "$container_id"
  done

  docker image ls --quiet \
    --filter "label=com.docker.compose.project=$PROJECT_NAME" \
    --filter "label=com.docker.compose.service=$service"

  if [ "$service" = "db" ]; then
    docker image inspect --format '{{.Id}}' postgres:18-alpine 2>/dev/null || true
  fi
}

# 選択した1サービスのコンテナ・volume・imageを削除し、残存確認する。
cleanup_service() {
  service=$1
  volume_key=$(volume_key_for_service "$service")
  image_ids=$(service_image_ids "$service" | sort -u)

  echo
  printf 'Stopping and removing %s...\n' "$service"
  compose rm --stop --force --volumes "$service"

  volume_names=$(docker volume ls --quiet \
    --filter "label=com.docker.compose.project=$PROJECT_NAME" \
    --filter "label=com.docker.compose.volume=$volume_key")

  for volume_name in $volume_names; do
    docker volume rm "$volume_name"
  done

  for image_id in $image_ids; do
    docker image rm "$image_id"
  done

  remaining_containers=$(docker container ls --all --quiet \
    --filter "label=com.docker.compose.project=$PROJECT_NAME" \
    --filter "label=com.docker.compose.service=$service")
  if [ -n "$remaining_containers" ]; then
    printf 'Error: %s containers still remain:\n%s\n' \
      "$service" "$remaining_containers" >&2
    return 1
  fi

  remaining_volumes=$(docker volume ls --quiet \
    --filter "label=com.docker.compose.project=$PROJECT_NAME" \
    --filter "label=com.docker.compose.volume=$volume_key")
  if [ -n "$remaining_volumes" ]; then
    printf 'Error: %s volumes still remain:\n%s\n' \
      "$service" "$remaining_volumes" >&2
    return 1
  fi

  remaining_images=$(docker image ls --quiet \
    --filter "label=com.docker.compose.project=$PROJECT_NAME" \
    --filter "label=com.docker.compose.service=$service")
  if [ -n "$remaining_images" ]; then
    printf 'Error: %s images still remain:\n%s\n' \
      "$service" "$remaining_images" >&2
    return 1
  fi

  for image_id in $image_ids; do
    if docker image inspect "$image_id" >/dev/null 2>&1; then
      printf 'Error: An image previously used by %s still remains: %s\n' \
        "$service" "$image_id" >&2
      return 1
    fi
  done

  printf '%s cleanup verification completed.\n' "$service"
}

# 全選択時はCompose全体を削除し、一部選択時は対象サービスだけを削除する。
if [ "$SELECT_FRONTEND" = true ] &&
  [ "$SELECT_BACKEND" = true ] &&
  [ "$SELECT_DB" = true ]; then
  all_image_ids=""
  image_references=$(compose config --images)

  for image_reference in $image_references; do
    if image_id=$(docker image inspect --format '{{.Id}}' "$image_reference" 2>/dev/null); then
      all_image_ids="$all_image_ids $image_id"
    fi
  done

  echo
  echo "Stopping and removing the complete Compose project..."
  compose down --remove-orphans --rmi all --volumes

  remaining_resources=$(docker container ls --all --quiet \
    --filter "label=com.docker.compose.project=$PROJECT_NAME")
  remaining_resources="$remaining_resources$(docker volume ls --quiet \
    --filter "label=com.docker.compose.project=$PROJECT_NAME")"
  remaining_resources="$remaining_resources$(docker network ls --quiet \
    --filter "label=com.docker.compose.project=$PROJECT_NAME")"
  remaining_resources="$remaining_resources$(docker image ls --quiet \
    --filter "label=com.docker.compose.project=$PROJECT_NAME")"

  if [ -n "$remaining_resources" ]; then
    echo "Error: Compose resources still remain. Rebuild was not started." >&2
    exit 1
  fi

  for image_id in $all_image_ids; do
    if docker image inspect "$image_id" >/dev/null 2>&1; then
      printf 'Error: An image previously used by this project still remains: %s\n' \
        "$image_id" >&2
      exit 1
    fi
  done

  echo "Complete Compose cleanup verification succeeded."
else
  if [ "$SELECT_FRONTEND" = true ]; then
    cleanup_service frontend
  fi

  if [ "$SELECT_BACKEND" = true ]; then
    cleanup_service backend
  fi

  if [ "$SELECT_DB" = true ]; then
    cleanup_service db
  fi
fi

echo

# DBは配布imageをpullし、frontend/backendはDockerfileから再ビルドする。
if [ "$SELECT_DB" = true ]; then
  echo "Pulling the db image..."
  compose pull db
fi

if [ "$SELECT_BACKEND" = true ]; then
  echo "Rebuilding the backend image without cache..."
  compose build --no-cache --pull backend
fi

if [ "$SELECT_FRONTEND" = true ]; then
  echo "Rebuilding the frontend image without cache..."
  compose build --no-cache --pull frontend
fi

echo
echo "Starting selected services..."

# 未選択の依存サービスを変更しないよう、--no-depsで選択サービスだけを起動する。
if [ "$SELECT_DB" = true ]; then
  compose up --detach --no-deps --wait db
fi

if [ "$SELECT_BACKEND" = true ]; then
  compose up --detach --no-deps backend
fi

if [ "$SELECT_FRONTEND" = true ]; then
  compose up --detach --no-deps frontend
fi

echo
echo "Selected services were recreated successfully."
compose ps
