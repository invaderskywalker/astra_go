tree . -I "venv"

find astra -type f -name "*" | while read file; do   echo "====== $file ======";   cat "$file";   echo -e "\n"; done


find astra/agents/actions -type f -name "*" | while read file; do   echo "====== $file ======";   cat "$file";   echo -e "\n"; done

find . \
  -type d \( -name node_modules -o -name dist -o -name .git -o -name .next -o -name .cache \) -prune -o \
  -type f ! \( -name ".env" -o -name ".env.*" -o -name "package-lock.json" \) -print | \
while read -r file; do
  echo "====== $file ======"
  if file "$file" | grep -qE 'text|UTF-8'; then
    cat "$file"
  else
    echo "[Binary or non-text file skipped]"
  fi
  echo -e "\n"
done


1️⃣ Connect to your database
psql -h localhost -p 5432 -U postgres -d astra_main


You should see:

astra_main=#

2️⃣ Check if the extension exists
\dx


This lists all installed extensions. Look for uuid-ossp. If it’s not there, you need to install it.

3️⃣ Install the extension

Run:

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";


IF NOT EXISTS ensures it won’t fail if it’s already installed.

Once executed, uuid_generate_v4() will be available in your database.

4️⃣ Verify
\dx


You should now see something like:

               List of installed extensions
  Name    | Version |   Schema   |         Description
----------+---------+------------+----------------------------
uuid-ossp | 1.1     | public     | generate universally unique identifiers (UUIDs)




after installing - run 
minio server ~/minio-data --console-address ":9001" --address ":9000"

and then
 mc alias set 'myminio' 'http://192.168.1.27:9000' 'minioadmin' 'minioadmin'
 mc alias set local http://localhost:9000 minioadmin minioadmin
 
then i can use it in code.. 


go install golang.org/x/tools/cmd/godoc@latest
godoc -http=:8001


RELEASE
go build -o ./astra_cli ./astra/cmd
sudo mv ./astra_cli /usr/local/bin/astra

