tree . -I "venv"

find . -type f -name "*" | while read file; do   echo "====== $file ======";   cat "$file";   echo -e "\n"; done
