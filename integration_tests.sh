#!/bin/bash
set -e
PKGARGS="$*"

rm -rf covdatafiles
mkdir covdatafiles
export GOCOVERDIR=covdatafiles

# Start HTTP server in the background
python3 -m http.server 8000 &

# Store the PID of the server process
SERVER_PID=$!

# Wait for the server to start
# sleep 1

# Send a request to the server
# curl http://localhost:8000

go build -cover $BUILDARGS .

./devprivops schema attack-tree > res.json
cmp res.json schema/schemas/atk-tree-schema.json

./devprivops schema query > res.json
cmp res.json schema/schemas/query-schema.json

./devprivops schema report > res.json
cmp res.json schema/schemas/report_data-schema.json

./devprivops schema requirement > res.json
cmp res.json schema/schemas/requirement-schema.json

rm res.json

cnt=0
diff "test_files/expected_outputs/out_$cnt.txt" <(./devprivops analyse user pass 127.0.0.1 3030 tmp --report-endpoint http://localhost:8000 2>&1)
echo "================== TEST DONE!"
cnt=$((cnt + 1))
diff "test_files/expected_outputs/out_$cnt.txt" <(./devprivops test user pass 127.0.0.1 3030 tmp 2>&1)
echo "================== TEST DONE!"
cnt=$((cnt + 1))
diff "test_files/expected_outputs/out_$cnt.txt" <(./devprivops analyse user pass 127.0.0.1 3030 tmp --local-dir test_files/test_1 2>&1)
echo "================== TEST DONE!"
cnt=$((cnt + 1))
diff "test_files/expected_outputs/out_$cnt.txt" <(./devprivops analyse user pass 127.0.0.1 3030 tmp --local-dir test_files/test_2 2>&1)
echo "================== TEST DONE!"
cnt=$((cnt + 1))
diff "test_files/expected_outputs/out_$cnt.txt" <(./devprivops analyse user pass 127.0.0.1 3030 tmp --local-dir test_files/test_3 2>&1)
echo "================== TEST DONE!"
cnt=$((cnt + 1))
diff "test_files/expected_outputs/out_$cnt.txt" <(./devprivops analyse user pass 127.0.0.1 3030 tmp --local-dir test_files/test_4 2>&1)
echo "================== TEST DONE!"
cnt=$((cnt + 1))
diff "test_files/expected_outputs/out_$cnt.txt" <(./devprivops analyse user pass 127.0.0.1 3030 tmp --local-dir test_files/test_5 2>&1)
echo "================== TEST DONE!"
cnt=$((cnt + 1))
diff "test_files/expected_outputs/out_$cnt.txt" <(./devprivops analyse user pass 127.0.0.1 3030 tmp --local-dir test_files/test_6 2>&1)
echo "================== TEST DONE!"
cnt=$((cnt + 1))
diff "test_files/expected_outputs/out_$cnt.txt" <(./devprivops analyse user pass 127.0.0.1 3030 tmp --local-dir test_files/test_7 2>&1)
echo "================== TEST DONE!"
cnt=$((cnt + 1))
diff "test_files/expected_outputs/out_$cnt.txt" <(./devprivops test user pass 127.0.0.1 3030 tmp --local-dir test_files/test_7 2>&1)
echo "================== TEST DONE!"
cnt=$((cnt + 1))
# diff <(grep -vE "$timestamp_regex" "test_files/expected_outputs/out_$cnt.txt") <(./devprivops test user pass 127.0.0.1 3030 tmp --pipeline --local-dir test_files/test_7 2>&1 | grep -vE "$timestamp_regex")
diff <(sed 's/time=[^ ]* //g' "test_files/expected_outputs/out_$cnt.txt") <(./devprivops test user pass 127.0.0.1 3030 tmp --pipeline --local-dir test_files/test_7 2>&1 | sed 's/time=[^ ]* //g')
echo "================== TEST DONE!"

# Close the server
kill $SERVER_PID

rm devprivops

go tool covdata percent -i=covdatafiles
# go tool covdata func -i=covdatafiles
