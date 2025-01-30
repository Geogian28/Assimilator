#!/bin/bash
## Borrowed heavily from TechDufus: https://github.com/TechDufus/dotfiles/blob/main/bin/dotfiles

# Initialize Variables
GITHUB_ACCESS_TOKEN=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
ASSIMILATOR_DIR="$HOME/.assimilator"  # replaced from DOTFILES_DIR
ASSIMILATOR_LOG=$HOME/.assimilator.log  # replaced from DOTFILES_LOG
SSH_DIR="$HOME/.ssh"
IS_FIRST_RUN="$HOME/.assimilator_run"

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

# bash <(curl -H 'Authorization: token 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' -H 'Accept: application/vnd.github.v3.raw' -L https://api.github.com/repos/geogian28/Assimilator/contents/assimilator.sh)
# curl -H 'Authorization: token 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' -H 'Accept: application/vnd.github.v3.raw' -L https://api.github.com/repos/geogian28/Assimilator/contents/assimilator.sh

#CURL_COMMAND /helloworld.yml
#bash -c "$(CURL_COMMAND /Scripts/new_machine_setup.sh)"

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
  # empty conduro.log
  > $ASSIMILATOR_LOG
  # hide stdout, on error we print and exit
  if eval "$1" 1> /dev/null 2> $ASSIMILATOR_LOG; then
    return 0 # success
  fi
  # read error from log and add spacing
  printf "${OVERWRITE}${LRED} [X]  ${TASK}${LRED}\n"
  while read line; do
    printf "      ${line}\n"
  done < $ASSIMILATOR_LOG
  printf "\n"
  # remove log file
  rm $ASSIMILATOR_LOG
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
    _cmd "sudo apt-get update"
    _cmd "sudo apt-get install -y software-properties-common"
    _cmd "sudo apt-add-repository -y ppa:ansible/ansible"
    _cmd "sudo apt-get update"
    _cmd "sudo apt-get install -y ansible"
    _cmd "sudo apt-get install python3-argcomplete"
    __task "Installing Git"
    _cmd "sudo apt-get install git -y"
    _cmd "sudo activate-global-python-argcomplete3"
  fi
  if ! dpkg -s python3 >/dev/null 2>&1; then
    __task "Installing Python3"
    _cmd "sudo apt-get install -y python3"
  fi
}

function redhat_setup() {
  if ! yum list installed | grep ansible >/dev/null 2>&1; then
    __task "Installing Ansible"
    _cmd "sudo yum update -y"
    _cmd "sudo yum install -y ansible"
    _cmd "sudo yum install -y python3-argcomplete"
    _cmd "sudo activate-global-python-argcomplete"
    __task "Installing Git "
    _cmd "sudo yum install git -y"
  fi
  #if ! yum list installed | grep python3 >/dev/null 2>&1; then
  #  __task "Installing Python3"
  #  _cmd "sudo yum install -y python3"
  #fi
}

OS_FAMILY=""
if [ -f /usr/bin/apt ]; then
   OS_FAMILY="debian"
   ubuntu_setup
fi
if [ -f /usr/bin/yum ]; then
    OS_FAMILY="fedora"
    redhat_setup
fi


if ! [[ -d "$ASSIMILATOR_DIR" ]]; then
  __task "Cloning repository"
  # _cmd "git clone https://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@github.com/Geogian28/Assimilator $ASSIMILATOR_DIR"
  git clone https://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@github.com/Geogian28/Assimilator $ASSIMILATOR_DIR
else
  __task "Updating repository"
  git -C $ASSIMILATOR_DIR pull --quiet > /dev/null
fi
#__task "Running Ansible Playbook"
# ansible-playbook "$DOTFILES_DIR/main.yml" "$@"
sudo ansible-playbook $ASSIMILATOR_DIR/main.yaml
# ansible-playbook $(CURL_COMMAND "/Scripts/NewMachineSetup/main.yaml")