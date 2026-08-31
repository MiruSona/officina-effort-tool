# 시험 자료

여기 있는 JSONL 은 **실물이 아니다.** 실제 `~/.claude/projects` 조각을 아래 여섯 규칙으로
지운 뒤 손으로 새로 쓴 것이다. 새 조각을 넣을 때도 이 규칙을 지킨다.

1. `message.content` 는 통째로 지우고 `"[삭제]"` 로 바꾼다. 재는 것은 `usage` 와 시각뿐이다.
2. `cwd` · `gitBranch` · 파일 경로는 `C:\proj` 같은 가짜로 바꾼다. 사용자 이름·메일·집 경로 금지.
3. `sessionId` · `uuid` · `requestId` · `message.id` 는 모양만 지키고 값을 새로 만든다 (`req_test0001` 등).
4. `meta.json` 의 `description` 은 분류 시험에 필요하므로 분류 낱말만 남긴 가짜 문장으로 새로 쓴다.
5. 넣기 전에 `sk-` · `ghp_` · `AKIA` · `@` 를 한 번 훑고, 걸리면 그 조각을 안 쓴다.
6. 시험은 `t.TempDir()` 사본에만 돌린다. 실제 `~/.claude` 를 가리키는 시험을 만들지 않는다.

## 자리

| 폴더 | 무엇 |
| --- | --- |
| `sessions/small/` | 작은 세션 하나. JSONL 합이 `cost-state` 와 딱 맞는다 |
| `sessions/nosubagents/` | `subagents/` 폴더가 없는 옛 세션 |
| `sessions/inprogress/` | `cost-state` 줄이 없는 도는 중 세션 |
| `notify/` | 서브에이전트 완료 알림이 섞인 세션. 이어받기·도구 세기 시험 |
| `rules/ok.txt` · `rules/bad.txt` | 규칙 파일 문법 시험 |

`sessions/` 와 `notify/` 는 각각 통째로 `--projects` 뿌리로 쓴다. 그 아래 폴더 하나가 프로젝트 하나다.
**뿌리를 나눈 까닭** : `sessions/` 를 세는 회귀 시험(작업 수·안 바뀜 개수)이 있어서, 세션을 더하면 그
시험들이 깨진다.

`notify/` 의 `message.content` 는 분류 시험에 제목이 필요해서 **규칙 4를 따라 분류 낱말만 남긴 가짜
문장**으로 새로 썼다. 사람 이름·경로·비밀값은 없다.
