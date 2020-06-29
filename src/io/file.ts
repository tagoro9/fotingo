import editor from 'editor';
import * as fs from 'fs';
import { resolve } from 'path';
import { zipObj } from 'ramda';
import * as tmp from 'tmp';
import { promisify } from 'util';

interface EditVirtualFileOptions {
  // Extension the file should have
  extension: string;
  // Initial content of the file
  initialContent?: string;
  // Prefix to include in the file name
  prefix: string;
}

interface FileData {
  // Function to be invoked when done when working with the file
  clean: () => void;
  // File descriptor
  fd: string;
  // Path to tmp file
  path: string;
}

const readFile = promisify(fs.readFile);
const writeFile = promisify(fs.writeFile);

/**
 * Promisified version of tmp.file
 * @param fileOptions File creation options
 */
function createTemporaryFile(fileOptions: tmp.FileOptions): Promise<FileData> {
  return new Promise((resolve, reject) => {
    tmp.file(fileOptions, (error, ...temporaryFileArguments) => {
      if (error) {
        return reject(error);
      }
      resolve((zipObj(['path', 'fd', 'clean'], temporaryFileArguments) as unknown) as FileData);
    });
  });
}

/**
 * Open users preferred editor to edit a file and return the file
 * contents. If the process exits with a non zero code or it is killed
 * by a signal the function will throw an error
 * @param path Path fo the file to edit
 */
async function editFile(path: string): Promise<string> {
  return new Promise((resolve, reject) => {
    editor(path, (code?: number, signal?: string) => {
      if (code !== 0 || signal !== null) {
        return reject(new Error('The editor exited with an error code'));
      }
      resolve(readFile(path, 'utf-8'));
    });
  });
}

/**
 * Allow user to input structured content by creating a tmp file and
 * opening their preferred editor. The function resolves the contents of the file
 * @param options Editing options
 */
export async function editVirtualFile(options: EditVirtualFileOptions): Promise<string> {
  const fileData = await createTemporaryFile({
    postfix: `.${options.extension}`,
    prefix: options.prefix,
  });
  await writeFile(fileData.fd, options.initialContent || '');
  const fileContent = await editFile(fileData.path);
  fileData.clean();
  return fileContent;
}

/**
 * Get the file content of the file name if it exists in any of the specified folders
 * @param name File name
 * @param root Root folder where to start search
 * @param folders Folders where to look for the file
 */
export async function getFileContent(
  name: string,
  root: string,
  folders: string[],
): Promise<string | undefined> {
  const data = await Promise.all(
    folders
      .map((folder) => resolve(root, folder, name))
      .map((p) => readFile(p, 'utf-8').catch(() => undefined)),
  );
  return data.filter((error) => error !== undefined)[0];
}
