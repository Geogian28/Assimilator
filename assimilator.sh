#!/bin/bash
## Borrowed heavily from TechDufus: https://github.com/TechDufus/dotfiles/blob/main/bin/dotfiles

# Check if the script is being run with root privileges.
# If not, exit with an error message.
if [[ $EUID -ne 0 ]]; then
  if command -v sudo >/dev/null 2>&1; then
    echo "Attempting to escalate to root privileges..."
    sudo "$0" "$@" # Re-execute the script with sudo
    exit $? # Exit with the sudo exit code
  else
    echo "Error: This script requires root privileges, but sudo is not available."
    exit 1 # Exit with an error code
  fi
fi

# Initialize Variables
test_mode=false
GITHUB_ACCESS_TOKEN=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
ASSIMILATOR_DIR="/etc/assimilator"
ASSIMILATOR_LOG="/var/log/assimilator.log"
LOGROTATE_CONF="/etc/logrotate.d/assimilator" # Not yet implemented
IS_FIRST_RUN="$ASSIMILATOR_DIR/assimilator_run"

# Initialize arguments
# Use getopt to parse options
options=$(getopt -o "" -l "test" -- "$@")
eval set -- "$options"
while true; do
  case "$1" in
    --test)
      test_mode=true
      shift ;;
    --)
      shift
      break ;;
    *)
      break ;;
  esac
done
if [ "$1" = "test" ]; then test_mode=true; fi

# color codes
RESTORE='\033[0m'
NC='\033[0m'
BLACK='\033[00;30m'
RED='\033[00;31m'
GREEN='\033[00;32m'
YELLOW='\033[00;33m'
BLUE='\033[00;34m'
PURPLE='\033[00;35m'
CYAN='\033[00;36m'
SEA="\\033[38;5;49m"
LIGHTGRAY='\033[00;37m'
LBLACK='\033[01;30m'
LRED='\033[01;31m'
LGREEN='\033[01;32m'
LYELLOW='\033[01;33m'
LBLUE='\033[01;34m'
LPURPLE='\033[01;35m'
LCYAN='\033[01;36m'
WHITE='\033[01;37m'
OVERWRITE='\e[1A\e[K'

#emoji codes
CHECK_MARK="${GREEN}\xE2\x9C\x94${NC}"
X_MARK="${RED}\xE2\x9C\x96${NC}"
PIN="${RED}\xF0\x9F\x93\x8C${NC}"
CLOCK="${GREEN}\xE2\x8C\x9B${NC}"
ARROW="${SEA}\xE2\x96\xB6${NC}"
BOOK="${RED}\xF0\x9F\x93\x8B${NC}"
HOT="${ORANGE}\xF0\x9F\x94\xA5${NC}"
WARNING="${RED}\xF0\x9F\x9A\xA8${NC}"
RIGHT_ANGLE="${GREEN}\xE2\x88\x9F${NC}"

function CURL_COMMAND() {
    curl -H 'Authorization: token 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' \
    -H 'Accept: application/vnd.github.v3.raw' \
    -L https://api.github.com/repos/geogian28/Assimilator/contents$1
}

## This works MAYBE!?!?!?
# bash <(curl -H 'Authorization: token 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' -H 'Accept: application/vnd.github.v3.raw' -L https://api.github.com/repos/geogian28/Assimilator/contents/assimilator.sh)


## Unsure about this
# curl -H 'Authorization: token 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' -H 'Accept: application/vnd.github.v3.raw' -L https://api.github.com/repos/geogian28/Assimilator/contents/assimilator.sh | bash

function __task {
  # if _task is called while a task was set, complete the previous
  if [[ $TASK != "" ]]; then
    printf "${OVERWRITE}${LGREEN} [✓]  ${LGREEN}${TASK}\n"
  fi
  # set new task title and print
  TASK=$1
  printf "${LBLACK} [ ]  ${TASK} \n${LRED}"
}

# _cmd performs commands with error checking
function _cmd {
  #create log if it doesn't exist
  if ! [[ -f $ASSIMILATOR_LOG ]]; then
    touch $ASSIMILATOR_LOG
  fi
  # hide stdout, on error we print and exit
  if eval "$1" 1> /dev/null 2>> "$ASSIMILATOR_LOG" ; then
    return 0 # success
  fi
  # read error from log and add spacing
  printf "${OVERWRITE}${LRED} [X]  ${TASK}${LRED}\n"
  while read line; do
   printf "      ${line}\n"
  done < $ASSIMILATOR_LOG
  printf "\n"
  # exit installation
  exit 1
}

function _clear_task {
  TASK=""
}

function _task_done {
  printf "${OVERWRITE}${LGREEN} [✓]  ${LGREEN}${TASK}\n"
  _clear_task
}

function ubuntu_setup() {
  if ! dpkg -s ansible >/dev/null 2>&1; then
    __task "Installing Ansible"
    _cmd "apt-get update"
    _cmd "apt-get install -y software-properties-common"
    _cmd "apt-get install -y zsh"
    _cmd "apt-add-repository -y ppa:ansible/ansible"
    _cmd "apt-get update"
    _cmd "apt-get install -y ansible"
    _cmd "apt-get install python3-argcomplete"
    __task "Installing Git"
    _cmd "apt-get install git -y"
  fi
  if ! dpkg -s python3 >/dev/null 2>&1; then
    __task "Installing Python3"
    _cmd "apt-get install -y python3"
    _cmd "activate-global-python-argcomplete3"
  fi
}

function redhat_setup() {
  if ! yum list installed | grep ansible >/dev/null 2>&1; then
    __task "Installing Ansible"
    _cmd "yum update -y"
    _cmd " install -y ansible"
    _cmd "yum install -y python3-argcomplete"
    _cmd "activate-global-python-argcomplete"
    _task_done
    __task "Installing Git "
    _cmd "yum install git -y"
    _task_done
  fi
  #if ! yum list installed | grep python3 >/dev/null 2>&1; then
  #  __task "Installing Python3"
  #  _cmd "sudo yum install -y python3"
  #fi
}

#if ! [[ -d "$ASSIMILATOR_DIR" ]]; then
#  __task "Creating Assimilator directory"
#  sudo mkdir -p "$ASSIMILATOR_DIR"
#  sudo chmod 755 $ASSIMILATOR_DIR
#  _task_done
#fi

if ! [[ -f $LOGROTATE_CONF ]]; then
  __task "Creating logrotate config"
  # touch "$LOGROTATE_CONF"
  echo "/var/log/my_script.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
    }" > "$LOGROTATE_CONF"
fi

echo "" >> $ASSIMILATOR_LOG
echo "##########################" 1>> $ASSIMILATOR_LOG
echo "### Installing Assimilator" 1>> $ASSIMILATOR_LOG

OS_FAMILY=""
if [ -f /usr/bin/apt ]; then
   OS_FAMILY="debian"
   ubuntu_setup
fi
if [ -f /usr/bin/yum ]; then
    OS_FAMILY="fedora"
    redhat_setup
fi

if [[ "$test_mode" == true ]]; then
__task "Testing Assimilator"
  mkdir -p "$ASSIMILATOR_DIR"
  cp -R /mnt/nfs/GitRepos/Assimilator/* "$ASSIMILATOR_DIR"
  #sudo ansible-playbook "$ASSIMILATOR_DIR/main.yaml"
  _task_done
  #exit 0
fi


if ! [[ -d "$ASSIMILATOR_DIR" ]]; then
  __task "Cloning repository"
  _cmd "git clone https://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@github.com/Geogian28/Assimilator $ASSIMILATOR_DIR"
  _task_done
  exit
fi
git --git-dir=/etc/assimilator/.git --work-tree=/etc/assimilator/.git fetch
# if ! git --git-dir=/etc/assimilator/.git --work-tree=/etc/assimilator/.git diff --quiet ; then
#   echo "Updating Assimilator"
#   __task "Updating Assimilator"
#   _cmd "git -C $ASSIMILATOR_DIR pull --quiet > /dev/null"
#   _task_done
#   exit
# else 
#   __task "Assimilator is up to date"
#   _task_done
# fi

__task "Running Ansible Playbook"
ansible-playbook "$ASSIMILATOR_DIR/main.yaml" -i "$ASSIMILATOR_DIR/inventory.ini" 2> >(tee -a $ASSIMILATOR_LOG)
