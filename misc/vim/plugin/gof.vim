if executable('gof') && get(g:, 'gof_default_commands', 1)
  command! -nargs=0 Gof call gof#run('edit')
  command! -nargs=0 GofS call gof#run('split')
  command! -nargs=0 GofV call gof#run('vsplit')
endif
