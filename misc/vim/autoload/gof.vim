let s:gof_launcher = get(g:, 'gof_launcher', has("win32") ? '' : 'xterm -e')

function! gof#run(edit)
  if has("gui_running")
    let tmp = tempname()
    silent exe printf("!%s gof > %s", s:gof_launcher, tmp)
    let files = readfile(tmp)
    call delete(tmp)
  else
    let files = split(system("gof"), "\n")
  endif
  redraw!
  if len(files) > 0
    for file in files
      exe a:edit file
    endfor
  endif
endfunction
