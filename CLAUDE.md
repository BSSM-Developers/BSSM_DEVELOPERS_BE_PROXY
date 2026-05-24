# CLAUDE.md

This file provides guidance to Claude Code when working with code in this repository.

## Project Overview

BSSM Developers API Proxy — Go 기반 API 프록시 서비스.
외부 API 호출을 중계하며 인증, Rate Limiting, 요청 큐, 로깅을 담당한다.

원본 Java/Spring WebFlux 구현체(`bssm-dev/proxy-service`)를 Go로 재작성.

## Build & Run

```bash
go build -o proxy ./cmd/...   # 빌드
./proxy                        # 실행
go test ./...                  # 전체 테스트
go test ./internal/...         # 내부 패키지 테스트
```

## Architecture

```
cmd/main.go                    # 엔트리포인트 & DI 루트
internal/
  apperrors/                   # 프록시 에러 타입 정의
  config/                      # Viper 기반 설정 로드
  domain/api/
    model/                     # 도메인 모델 (GORM 태그 포함)
    repository/                # MySQL GORM 레포지토리
    query/                     # 캐시 통합 쿼리 서비스
    service/                   # 비즈니스 로직 (Pipeline, Browser/Server, RateLimit)
    handler/                   # Gin HTTP 핸들러
  cache/                       # 2레벨 캐시 (L1: go-cache, L2: Redis)
  validator/                   # SSRF 방어 도메인 검증
  requester/                   # 외부 API HTTP 클라이언트 (풀링, 헤더 필터)
  queue/                       # HRN 우선순위 요청 큐
  log/                         # MongoDB 프록시 요청/응답 로그
  middleware/                  # Gin 미들웨어 (Queue, Error, CORS)
```

## Key Design Principles

- **SOLID / GRASP 준수**: 각 타입은 단일 책임, 의존성은 인터페이스로
- **DI 수동 주입**: `cmd/main.go`에서 명시적 생성자 체인
- **Context 전파**: 모든 I/O 함수 첫 번째 파라미터는 `context.Context`
- **에러 타입**: `*apperrors.ProxyError`로 상태 코드 포함

## Commit Convention

```
feat :: 기능명
fix :: 수정내용
refactor :: 리팩터링 내용
perf :: 성능 개선
chore :: 설정/빌드
```

## Configuration

환경 변수 또는 `config.yaml` 파일로 설정 (Viper).
예시: `internal/config/config.go` 참조.

## Security Notes

- SSRF 방어: `internal/validator/domain_validator.go` — https only, 내부 IP 차단, DNS 검증
- 리다이렉트 비활성화: `internal/requester/http_requester.go`
- 헤더 필터: 요청 시 `bssm-dev-token`, `bssm-dev-secret`, `host` 제거
- 응답 Content-Type 허용 목록: `internal/requester/response_sanitizer.go`
