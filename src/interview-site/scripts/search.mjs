export function filterQuestions(root,value){
  const query=value.trim().toLocaleLowerCase();let count=0;
  for(const item of root.querySelectorAll('[data-question]')){item.hidden=!item.dataset.search.includes(query);if(!item.hidden)count++;}
  for(const section of root.querySelectorAll('[data-topic]'))section.hidden=![...section.querySelectorAll('[data-question]')].some(item=>!item.hidden);
  root.querySelector('#search-result').textContent=query?`找到 ${count} 题`:`全部 ${root.querySelectorAll('[data-question]').length} 题`;
  root.querySelector('#no-results').hidden=count!==0;
  return count;
}
