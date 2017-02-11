'use strict';

const plumber = require('gulp-plumber');
const newer = require('gulp-newer');
const babel = require('gulp-babel');
const watch = require('gulp-watch');
const gutil = require('gulp-util');
const gulp = require('gulp');
const path = require('path');
const fs = require('fs');

gulp.task('default', ['build']);

gulp.task('build', () => {
  return gulp.src('src/**/*')
    .pipe(plumber({
      errorHandler(err) {
        gutil.log(err.stack);
      },
    }))
    .pipe(newer('lib'))
    .pipe(babel())
    .pipe(gulp.dest('lib'));
});

gulp.task('watch', ['build'], () => {
  watch('src/**/*', () => {
    gulp.start('build');
  });
});
