#!/bin/bash
# End-to-end smoke test for the new features. Self-contained: builds, starts,
# tests and kills the server.
set -u
cd "$(dirname "$0")/.."

export PATH="/d/Go/bin:$PATH"
PORT=19527
BASE="http://localhost:$PORT"
TESTDIR=".devtest"
rm -rf "$TESTDIR"
mkdir -p "$TESTDIR/uploads"

cat > "$TESTDIR/config.json" <<EOF
{"port": "$PORT", "sqlitePath": "$TESTDIR/test.db", "jwtSecret": "e2e_secret", "uploadDir": "$TESTDIR/uploads"}
EOF

echo "== build =="
go build -o "$TESTDIR/chirp-server.exe" ./cmd/server || exit 1

echo "== start server =="
CONFIG_FILE="$TESTDIR/config.json" "./$TESTDIR/chirp-server.exe" > "$TESTDIR/server.log" 2>&1 &
SRV=$!
cleanup() { kill "$SRV" 2>/dev/null; taskkill //F //T //PID "$SRV" 2>/dev/null; }
trap cleanup EXIT
for i in $(seq 1 30); do curl -sf "$BASE/api/drive/quota" -o /dev/null 2>&1; [ $? -ne 0 ] || break; curl -s "$BASE/login" -o /dev/null 2>&1 && break; sleep 0.5; done
sleep 1

PASS=0; FAIL=0
check() { # name condition
  if [ "$2" = "0" ]; then PASS=$((PASS+1)); echo "  PASS  $1"; else FAIL=$((FAIL+1)); echo "  FAIL  $1"; fi
}

echo "== auth =="
curl -sf -X POST "$BASE/signup" -H 'Content-Type: application/json' -d '{"name":"E2E","email":"e2e@test.com","password":"pass1234"}' > /dev/null
check "signup" $?
TOKEN=$(curl -s -X POST "$BASE/login" -H 'Content-Type: application/json' -d '{"email":"e2e@test.com","password":"pass1234"}' | sed -E 's/.*"token":"([^"]+)".*/\1/')
[ -n "$TOKEN" ]; check "login token" $?
AUTH="Authorization: Bearer $TOKEN"

echo "== upload + instant (秒传) =="
head -c 300000 /dev/urandom > "$TESTDIR/blob.bin"
HASH=$(sha256sum "$TESTDIR/blob.bin" | cut -d' ' -f1)
R1=$(curl -s -X POST "$BASE/api/drive/files" -H "$AUTH" -F "file=@$TESTDIR/blob.bin;filename=first.bin")
echo "$R1" | grep -q '"id"'; check "upload first.bin" $?
F1_ID=$(echo "$R1" | sed -E 's/.*"id":([0-9]+).*/\1/')
F1_STORED=$(echo "$R1" | sed -E 's/.*"filename":"([^"]+)".*/\1/')

R2=$(curl -s -X POST "$BASE/api/drive/files/instant" -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"name\":\"second.bin\",\"hash\":\"$HASH\",\"size\":300000,\"folder_id\":null}")
echo "$R2" | grep -q '"id"'; check "instant upload second.bin" $?
F2_STORED=$(echo "$R2" | sed -E 's/.*"filename":"([^"]+)".*/\1/')
[ "$F1_STORED" = "$F2_STORED" ]; check "dedup shares physical object" $?
COUNT=$(ls "$TESTDIR/uploads" | grep -cv chunks)
[ "$COUNT" = "1" ]; check "only one physical file on disk" $?

echo "== version history =="
head -c 100000 /dev/urandom > "$TESTDIR/blob2.bin"
R3=$(curl -s -X POST "$BASE/api/drive/files" -H "$AUTH" -F "file=@$TESTDIR/blob2.bin;filename=first.bin")
V=$(echo "$R3" | sed -E 's/.*"version":([0-9]+).*/\1/')
[ "$V" = "2" ]; check "same-name upload becomes v2" $?
VERS=$(curl -s "$BASE/api/drive/files/$F1_ID/versions" -H "$AUTH")
echo "$VERS" | grep -q '"version":2'; check "versions list has v2" $?
V1_ID=$(echo "$VERS" | sed -E 's/.*"id":([0-9]+)[^}]*"version":1.*/\1/')
R4=$(curl -s -X POST "$BASE/api/drive/files/$F1_ID/versions/$V1_ID/restore" -H "$AUTH")
echo "$R4" | grep -q '"version":3'; check "restore version creates v3" $?

echo "== inline preview =="
echo "hello preview" > "$TESTDIR/note.txt"
RP=$(curl -s -X POST "$BASE/api/drive/files" -H "$AUTH" -F "file=@$TESTDIR/note.txt")
P_ID=$(echo "$RP" | sed -E 's/.*"id":([0-9]+).*/\1/')
CT=$(curl -s -D - -o /dev/null "$BASE/api/drive/files/$P_ID/download?inline=1" -H "$AUTH" | grep -i content-type)
echo "$CT" | grep -qi "text/plain"; check "inline content-type text/plain" $?
CD=$(curl -s -D - -o /dev/null "$BASE/api/drive/files/$P_ID/download?inline=1" -H "$AUTH" | grep -i content-disposition)
echo "$CD" | grep -qi "inline"; check "inline disposition" $?

echo "== share link =="
RS=$(curl -s -X POST "$BASE/api/drive/shares" -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"resource_id\":$P_ID,\"password\":\"8866\",\"expire_days\":7}")
STOKEN=$(echo "$RS" | sed -E 's/.*"token":"([^"]+)".*/\1/')
[ -n "$STOKEN" ]; check "create share" $?
INFO=$(curl -s "$BASE/api/shares/$STOKEN")
echo "$INFO" | grep -q '"file_name":"note.txt"'; check "share info public" $?
! echo "$INFO" | grep -q '"password":'; check "password not leaked" $?
WRONG=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/api/shares/$STOKEN/download?password=0000")
[ "$WRONG" = "403" ]; check "wrong extraction code 403" $?
RIGHT=$(curl -s "$BASE/api/shares/$STOKEN/download?password=8866")
[ "$RIGHT" = "hello preview" ]; check "share download with code" $?

echo "== batch ops =="
B1=$(curl -s -X POST "$BASE/api/drive/files" -H "$AUTH" -F "file=@$TESTDIR/note.txt;filename=b1.txt" | sed -E 's/.*"id":([0-9]+).*/\1/')
B2=$(curl -s -X POST "$BASE/api/drive/files" -H "$AUTH" -F "file=@$TESTDIR/note.txt;filename=b2.txt" | sed -E 's/.*"id":([0-9]+).*/\1/')
curl -s -X POST "$BASE/api/drive/batch/download" -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"file_ids\":[$B1,$B2]}" -o "$TESTDIR/batch.zip"
unzip -l "$TESTDIR/batch.zip" 2>/dev/null | grep -q "b1.txt"; check "batch zip contains b1.txt" $?
BM=$(curl -s -X POST "$BASE/api/drive/batch/delete" -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"file_ids\":[$B1,$B2],\"folder_ids\":[]}")
echo "$BM" | grep -q '"deleted":2'; check "batch delete 2 files" $?

echo "== chunked upload =="
head -c 20000000 /dev/urandom > "$TESTDIR/big.bin"
CHASH=$(sha256sum "$TESTDIR/big.bin" | cut -d' ' -f1)
INIT=$(curl -s -X POST "$BASE/api/drive/uploads/init" -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"filename\":\"big.bin\",\"size\":20000000,\"folder_id\":null,\"file_hash\":\"$CHASH\",\"chunk_size\":8000000}")
SID=$(echo "$INIT" | sed -E 's/.*"session":\{"id":"([^"]+)".*/\1/')
[ -n "$SID" ] && [ "$SID" != "$INIT" ]; check "upload session created" $?
# upload 3 chunks: 8MB + 8MB + 3.2MB
split -b 8000000 -d "$TESTDIR/big.bin" "$TESTDIR/chunk_"
for i in 0 1 2; do
  curl -s -o /dev/null -w "" -X PUT "$BASE/api/drive/uploads/$SID/chunks/$i" -H "$AUTH" --data-binary "@$TESTDIR/chunk_0$i"
  check "chunk $i uploaded" $?
done
STATE=$(curl -s "$BASE/api/drive/uploads/$SID" -H "$AUTH")
echo "$STATE" | grep -q '"uploaded_chunks":\[0,1,2\]'; check "session reports uploaded chunks" $?
DONE=$(curl -s -X POST "$BASE/api/drive/uploads/$SID/complete" -H "$AUTH")
echo "$DONE" | grep -q '"original_name":"big.bin"'; check "chunked complete" $?
echo "$DONE" | grep -q "\"file_hash\":\"$CHASH\""; check "merged hash matches" $?

echo "== activities =="
ACTS=$(curl -s "$BASE/api/activities?limit=100" -H "$AUTH")
echo "$ACTS" | grep -q '"action":"instant_upload"'; check "activity: instant_upload" $?
echo "$ACTS" | grep -q '"action":"share_create"'; check "activity: share_create" $?
echo "$ACTS" | grep -q '"action":"chunked_upload"'; check "activity: chunked_upload" $?
echo "$ACTS" | grep -q '"action":"restore_version"'; check "activity: restore_version" $?

echo ""
echo "== RESULT: $PASS passed, $FAIL failed =="
exit $FAIL
