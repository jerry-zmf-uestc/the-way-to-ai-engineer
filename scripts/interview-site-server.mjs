import {startBookDev} from './dev-book.mjs';
import {watchInterviewSite} from './watch-interview-site.mjs';
try{
 const [command='dev',...args]=process.argv.slice(2);
 if(!['dev','preview'].includes(command))throw new Error('Expected dev or preview');
 const port=command==='dev'?'4322':'4323';
 const session=startBookDev({command,watcherFactory:watchInterviewSite,args:['--config','astro.interview.config.mjs','--port',port,...args]});
 const result=await session.done;process.exitCode=result.code;
 if(result.error)process.stderr.write(`interview site: ${result.error.message}\n`);
}catch(error){console.error(error.message);process.exitCode=1;}
